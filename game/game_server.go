package game

import (
	"context"
	"encoding/json"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/common"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/db/mongodb"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"os"
	"sync"
	"time"
)

// 简化类名
type ProtoPacket = gnet.ProtoPacket
type Cmd = gnet.PacketCommand
type Connection = gnet.Connection
var (
	LogDebug = gnet.LogDebug
	LogError = gnet.LogError

	// singleton
	gameServer *GameServer
)

// 游戏服
type GameServer struct {
	common.BaseServer
	config *GameServerConfig
	// 玩家数据接口
	playerDb db.PlayerDb
	// 在线玩家
	playerMap sync.Map // playerId-*Player
}

// 游戏服配置
type GameServerConfig struct {
	common.BaseServerConfig
}

func GetServer() *GameServer {
	return gameServer
}

func (this *GameServer) GetPlayerDb() db.PlayerDb {
	return this.playerDb
}

func (this *GameServer) Init(configFile string) bool {
	gameServer = this
	if !this.BaseServer.Init(configFile) {
		return false
	}
	this.readConfig()
	this.initDb()
	this.initCache()

	netMgr := gnet.GetNetMgr()
	// 监听客户端
	clientCodec := gnet.NewProtoCodec(nil)
	clientHandler := NewClientConnectionHandler(clientCodec)
	this.registerClientPacket(clientHandler)
	if netMgr.NewListener(this.config.ClientListenAddr, this.config.ClientConnConfig, clientCodec, clientHandler, &ClientListerHandler{}) == nil {
		panic("listen client failed")
		return false
	}

	// 监听服务器
	serverCodec := gnet.NewProtoCodec(nil)
	serverHandler := gnet.NewDefaultConnectionHandler(serverCodec)
	this.registerServerPacket(serverHandler)
	if netMgr.NewListener(this.config.ServerListenAddr, this.config.ServerConnConfig, serverCodec, serverHandler, &ClientListerHandler{}) == nil {
		panic("listen server failed")
		return false
	}

	// 连接其他服务器
	this.BaseServer.SetDefaultServerConnectorConfig(this.config.ServerConnConfig)
	this.BaseServer.GetServerList().SetFetchAndConnectServerTypes("game")

	gnet.LogDebug("GameServer.Init")
	return true
}

func (this *GameServer) Run() {
	this.BaseServer.Run()
	LogDebug("GameServer.Run")
	this.BaseServer.WaitExit()
}

func (this *GameServer) OnExit() {
	LogDebug("GameServer.OnExit")
	if this.playerDb != nil {
		this.playerDb.(*mongodb.MongoDb).Disconnect()
	}
}

func (this *GameServer) readConfig() {
	fileData,err := os.ReadFile(this.GetConfigFile())
	if err != nil {
		panic("read config file err")
	}
	this.config = new(GameServerConfig)
	err = json.Unmarshal(fileData, this.config)
	if err != nil {
		panic("decode config file err")
	}
	gnet.LogDebug("%v", this.config)
	this.BaseServer.GetServerInfo().ServerId = this.config.ServerId
	this.BaseServer.GetServerInfo().ServerType = "game"
	this.BaseServer.GetServerInfo().ClientListenAddr = this.config.ClientListenAddr
	this.BaseServer.GetServerInfo().ServerListenAddr = this.config.ServerListenAddr
}

// 初始化数据库
func (this *GameServer) initDb() {
	// 使用mongodb来演示
	mongoDb := mongodb.NewMongoDb(this.config.MongoUri,"testdb","player")
	mongoDb.SetAccountColumnNames("accountid","")
	mongoDb.SetPlayerColumnNames("id", "name","regionid")
	if !mongoDb.Connect() {
		panic("connect db error")
	}
	this.playerDb = mongoDb
}

// 初始化redis缓存
func (this *GameServer) initCache() {
	cache.NewRedisClient(this.config.RedisUri, this.config.RedisPassword)
	pong,err := cache.GetRedis().Ping(context.TODO()).Result()
	if err != nil || pong == "" {
		panic("redis connect error")
	}
}

// 注册客户端消息回调
func (this *GameServer) registerClientPacket(clientHandler *ClientConnectionHandler) {
	clientHandler.Register(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), onHeartBeatReq, func() proto.Message {return &pb.HeartBeatReq{}})
	clientHandler.Register(gnet.PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameReq), onPlayerEntryGameReq, func() proto.Message {return &pb.PlayerEntryGameReq{}})
	clientHandler.autoRegisterPlayerComponentProto()
}

// 心跳回复
func onHeartBeatReq(connection gnet.Connection, packet *gnet.ProtoPacket) {
	req := packet.Message().(*pb.HeartBeatReq)
	connection.Send(Cmd(pb.CmdInner_Cmd_HeartBeatRes), &pb.HeartBeatRes{
		RequestTimestamp: req.GetTimestamp(),
		ResponseTimestamp: uint64(time.Now().UnixNano()/int64(time.Microsecond)),
	})
}

// 注册服务器消息回调
func (this *GameServer) registerServerPacket(serverHandler *gnet.DefaultConnectionHandler) {
	serverHandler.Register(Cmd(pb.CmdInner_Cmd_HeartBeatReq), onHeartBeatReq, func() proto.Message {return &pb.HeartBeatReq{}})
	//serverHandler.autoRegisterPlayerComponentProto()
}

// 添加一个在线玩家
func (this *GameServer) AddPlayer(player *Player) {
	this.playerMap.Store(player.GetId(), player)
	cache.AddOnlinePlayer(player.GetId(), gameServer.GetServerId())
}

// 删除一个在线玩家
func (this *GameServer) RemovePlayer(player *Player) {
	this.playerMap.Delete(player.GetId())
	cache.RemoveOnlineAccount(player.GetAccountId())
	cache.RemoveOnlinePlayer(player.GetId())
}

// 获取一个在线玩家
func (this *GameServer) GetPlayer(playerId int64) *Player {
	if v,ok := this.playerMap.Load(playerId); ok {
		return v.(*Player)
	}
	return nil
}
