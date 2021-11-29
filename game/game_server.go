package game

import (
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/common"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/db/mongodb"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
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
	// 玩家数据接口
	playerDb db.PlayerDb
	// 在线玩家
	playerMap sync.Map
}

// 游戏服配置
type GameServerConfig struct {
	// 客户端监听地址
	clientListenAddr string
	// 客户端连接配置
	clientConnConfig gnet.ConnectionConfig
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
	netMgr := gnet.GetNetMgr()
	clientCodec := gnet.NewProtoCodec(nil)
	clientHandler := gnet.NewDefaultConnectionHandler(clientCodec)
	this.registerClientPacket(clientHandler)
	if netMgr.NewListener(this.config.clientListenAddr, this.config.clientConnConfig, clientCodec, clientHandler, nil) == nil {
		panic("listen failed")
		return false
	}
	gnet.LogDebug("GameServer.Init")
	return true
}

func (this *GameServer) Run() {
	this.BaseServer.Run()
	gnet.LogDebug("GameServer.Run")
	this.BaseServer.WaitExit()
}

func (this *GameServer) OnExit() {
	gnet.LogDebug("GameServer.OnExit")
	if this.playerDb != nil {
		this.playerDb.(*mongodb.MongoDb).Disconnect()
	}
}

// 初始化数据库
func (this *GameServer) initDb() {
	// 使用mongodb来演示
	mongoDb := mongodb.NewMongoDb("mongodb://localhost:27017","testdb","player")
	mongoDb.SetAccountColumnNames("accountid","")
	mongoDb.SetPlayerColumnNames("id", "name","regionid")
	if !mongoDb.Connect() {
		panic("connect db error")
	}
	this.playerDb = mongoDb
}

func (this *GameServer) readConfig() {
	this.config = &GameServerConfig{
		clientListenAddr: "127.0.0.1:10003",
		clientConnConfig: gnet.ConnectionConfig{
			SendPacketCacheCap: 64,
			SendBufferSize:     1024 * 20,
			RecvBufferSize:     1024 * 10,
			MaxPacketSize:      1024 * 10,
			RecvTimeout:        10,
		},
	}
}

// 注册客户端消息回调
func (this *GameServer) registerClientPacket(clientHandler *gnet.DefaultConnectionHandler) {
	clientHandler.Register(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), onHeartBeatReq, func() proto.Message {return &pb.HeartBeatReq{}})
	clientHandler.Register(gnet.PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameReq), onPlayerEntryGameReq, func() proto.Message {return &pb.PlayerEntryGameReq{}})
	//clientHandler.Register(gnet.PacketCommand(pb.CmdLogin_Cmd_AccountReg), onAccountReg, func() proto.Message {return &pb.AccountReg{}})
}

// 客户端心跳回复
func onHeartBeatReq(connection gnet.Connection, packet *gnet.ProtoPacket) {
	req := packet.Message().(*pb.HeartBeatReq)
	connection.Send(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatRes), &pb.HeartBeatRes{
		RequestTimestamp: req.GetTimestamp(),
		ResponseTimestamp: uint64(time.Now().UnixNano()/int64(time.Microsecond)),
	})
}
