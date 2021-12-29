package game

import (
	"context"
	"encoding/json"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/common"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/db/mongodb"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"os"
	"sync"
	"time"
)

var (
	// singleton
	gameServer *GameServer
)

// 游戏服
type GameServer struct {
	common.BaseServer
	config *GameServerConfig
	// 服务器listener
	serverListener gnet.Listener
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

// 玩家db接口
func (this *GameServer) GetPlayerDb() db.PlayerDb {
	return this.playerDb
}

// 初始化
func (this *GameServer) Init(ctx context.Context, configFile string) bool {
	gameServer = this
	if !this.BaseServer.Init(ctx, configFile) {
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
	if netMgr.NewListener(ctx, this.config.ClientListenAddr, this.config.ClientConnConfig, clientCodec,
		clientHandler, &ClientListerHandler{}) == nil {
		panic("listen client failed")
		return false
	}

	// 监听服务器
	serverCodec := gnet.NewProtoCodec(nil)
	serverHandler := gnet.NewDefaultConnectionHandler(serverCodec)
	this.registerServerPacket(serverHandler)
	this.serverListener = netMgr.NewListener(ctx, this.config.ServerListenAddr, this.config.ServerConnConfig, serverCodec,
		serverHandler, nil)
	if this.serverListener == nil {
		panic("listen server failed")
		return false
	}

	// 连接其他服务器
	this.BaseServer.SetDefaultServerConnectorConfig(this.config.ServerConnConfig)
	this.BaseServer.GetServerList().SetFetchAndConnectServerTypes("game")

	logger.Debug("GameServer.Init")
	return true
}

// 运行
func (this *GameServer) Run(ctx context.Context) {
	this.BaseServer.Run(ctx)
	logger.Debug("GameServer.Run")
}

// 退出
func (this *GameServer) Exit() {
	this.BaseServer.Exit()
	logger.Debug("GameServer.Exit")
	if this.playerDb != nil {
		this.playerDb.(*mongodb.MongoDb).Disconnect()
	}
}

// 读取配置文件
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
	logger.Debug("%v", this.config)
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
	this.repairCache()
}

// 修复缓存,游戏服异常宕机重启后进行修复操作
func (this *GameServer) repairCache() {
	cache.ResetOnlinePlayer(this.GetServerId())
}

// 注册客户端消息回调
func (this *GameServer) registerClientPacket(clientHandler *ClientConnectionHandler) {
	// 手动注册消息回调
	clientHandler.Register(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), onHeartBeatReq, func() proto.Message {return &pb.HeartBeatReq{}})
	clientHandler.Register(gnet.PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameReq), onPlayerEntryGameReq, func() proto.Message {return &pb.PlayerEntryGameReq{}})
	// 通过反射自动注册消息回调
	clientHandler.autoRegisterPlayerComponentProto()
	// proto_code_gen工具生成的回调函数
	player_component_handler_auto_register(clientHandler)
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
	serverHandler.Register(Cmd(pb.CmdInner_Cmd_HeartBeatReq), onHeartBeatReq, func() proto.Message {return new(pb.HeartBeatReq)})
	serverHandler.Register(Cmd(pb.CmdInner_Cmd_KickPlayer), this.OnKickPlayer, func() proto.Message {return new(pb.KickPlayer)})
	//serverHandler.autoRegisterPlayerComponentProto()
}

// 添加一个在线玩家
func (this *GameServer) AddPlayer(player *Player) {
	this.playerMap.Store(player.GetId(), player)
	cache.AddOnlinePlayer(player.GetId(), player.GetAccountId(), gameServer.GetServerId())
}

// 删除一个在线玩家
func (this *GameServer) RemovePlayer(player *Player) {
	this.playerMap.Delete(player.GetId())
	cache.RemoveOnlineAccount(player.GetAccountId())
	cache.RemoveOnlinePlayer(player.GetId(), this.GetServerId())
}

// 获取一个在线玩家
func (this *GameServer) GetPlayer(playerId int64) *Player {
	if v,ok := this.playerMap.Load(playerId); ok {
		return v.(*Player)
	}
	return nil
}

// 踢玩家下线
func (this *GameServer) OnKickPlayer(connection gnet.Connection, packet *ProtoPacket) {
	req := packet.Message().(*pb.KickPlayer)
	player := this.GetPlayer(req.GetPlayerId())
	if player != nil {
		player.SetConnection(nil)
		player.Save()
		this.RemovePlayer(player)
	} else {
		logger.Error("kick player failed account:%v playerId:%v gameServerId:%v",
			req.GetAccountId(), req.GetPlayerId(), this.GetServerId())
		//// 有一种特殊情况: 玩家进入游戏服A,游戏服A宕机了,这时候redis缓存里面,依然记录着玩家还在游戏服A上
		//// 游戏服A重启后,当玩家重新登录时,登录服会通知游戏服A踢掉该玩家,但通过this.GetPlayer无法获取到玩家对象
		//// 所以这里判断如果缓存里面还是记录着玩家在这台服务器,就直接清理缓存
		//gameServerId := cache.GetOnlinePlayerGameServerId(req.GetPlayerId())
		//if gameServerId == this.GetServerId() {
		//	cache.RemoveOnlineAccount(req.GetAccountId())
		//	cache.RemoveOnlinePlayer(req.GetPlayerId(), this.GetServerId())
		//	LogError("kick player account:%v playerId:%v gameServerId:%v",
		//		req.GetAccountId(), req.GetPlayerId(), this.GetServerId())
		//}
	}
}
