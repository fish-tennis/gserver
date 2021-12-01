package game

import (
	"context"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/common"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/db/mongodb"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
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
	// 服务器id
	serverId int32
	// 客户端监听地址
	clientListenAddr string
	// 客户端连接配置
	clientConnConfig gnet.ConnectionConfig
	// mongodb地址
	mongoUri string
}

func GetServer() *GameServer {
	return gameServer
}

func (this *GameServer) GetPlayerDb() db.PlayerDb {
	return this.playerDb
}

func (this *GameServer) Init() bool {
	gameServer = this
	if !this.BaseServer.Init() {
		return false
	}
	this.readConfig()
	this.initDb()
	this.initCache()
	netMgr := gnet.GetNetMgr()
	clientCodec := gnet.NewProtoCodec(nil)
	clientHandler := NewClientConnectionHandler(clientCodec)
	this.registerClientPacket(clientHandler)
	if netMgr.NewListener(this.config.clientListenAddr, this.config.clientConnConfig, clientCodec, clientHandler, &ClientListerHandler{}) == nil {
		panic("listen failed")
		return false
	}
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
	this.config = &GameServerConfig{
		serverId: 101,
		clientListenAddr: "127.0.0.1:10003",
		clientConnConfig: gnet.ConnectionConfig{
			SendPacketCacheCap: 64,
			SendBufferSize:     1024 * 20,
			RecvBufferSize:     1024 * 10,
			MaxPacketSize:      1024 * 10,
			RecvTimeout:        10,
		},
		mongoUri: "mongodb://localhost:27017",
	}
	this.BaseServer.ServerId = this.config.serverId
}

// 初始化数据库
func (this *GameServer) initDb() {
	// 使用mongodb来演示
	mongoDb := mongodb.NewMongoDb(this.config.mongoUri,"testdb","player")
	mongoDb.SetAccountColumnNames("accountid","")
	mongoDb.SetPlayerColumnNames("id", "name","regionid")
	if !mongoDb.Connect() {
		panic("connect db error")
	}
	this.playerDb = mongoDb
}

// 初始化redis缓存
func (this *GameServer) initCache() {
	redisAddrs := []string{"10.0.75.2:6379"}
	cache.NewRedisClient(redisAddrs, "")
	pong,err := cache.GetRedis().Ping(context.TODO()).Result()
	if err != nil || pong == "" {
		panic("redis connect error")
	}
}

// 注册客户端消息回调
func (this *GameServer) registerClientPacket(clientHandler *ClientConnectionHandler) {
	clientHandler.Register(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), onHeartBeatReq, func() proto.Message {return &pb.HeartBeatReq{}})
	clientHandler.Register(gnet.PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameReq), onPlayerEntryGameReq, func() proto.Message {return &pb.PlayerEntryGameReq{}})
	//clientHandler.Register(gnet.PacketCommand(pb.CmdLogin_Cmd_AccountReg), onAccountReg, func() proto.Message {return &pb.AccountReg{}})
	clientHandler.autoRegisterPlayerComponentProto()
}

// 客户端心跳回复
func onHeartBeatReq(connection gnet.Connection, packet *gnet.ProtoPacket) {
	req := packet.Message().(*pb.HeartBeatReq)
	connection.Send(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatRes), &pb.HeartBeatRes{
		RequestTimestamp: req.GetTimestamp(),
		ResponseTimestamp: uint64(time.Now().UnixNano()/int64(time.Microsecond)),
	})
}

// 添加一个在线玩家
func (this *GameServer) AddPlayer(player *Player) {
	this.playerMap.Store(player.GetId(), player)
	cache.AddOnlinePlayer(player.GetId(), gameServer.ServerId)
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
