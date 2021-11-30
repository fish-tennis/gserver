package login

import (
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/common"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/db/mongodb"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"time"
)

var (
	// singleton
	loginServer *LoginServer
)

// 登录服
type LoginServer struct {
	common.BaseServer
	config *LoginServerConfig
	// 账号数据接口
	accountDb db.AccountDb
}

// 登录服配置
type LoginServerConfig struct {
	// 客户端监听地址
	clientListenAddr string
	// 客户端连接配置
	clientConnConfig gnet.ConnectionConfig
}

func (this *LoginServer) GetAccountDb() db.AccountDb {
	return this.accountDb
}

func (this *LoginServer) Init() bool {
	loginServer = this
	if !this.BaseServer.Init() {
		return false
	}
	this.readConfig()
	this.initDb()
	this.initCache()
	netMgr := gnet.GetNetMgr()
	clientCodec := gnet.NewProtoCodec(nil)
	clientHandler := gnet.NewDefaultConnectionHandler(clientCodec)
	this.registerClientPacket(clientHandler)
	if netMgr.NewListener(this.config.clientListenAddr, this.config.clientConnConfig, clientCodec, clientHandler, nil) == nil {
		panic("listen failed")
		return false
	}
	gnet.LogDebug("LoginServer.Init")
	return true
}

func (this *LoginServer) Run() {
	this.BaseServer.Run()
	gnet.LogDebug("LoginServer.Run")
	this.BaseServer.WaitExit()
}

func (this *LoginServer) OnExit() {
	gnet.LogDebug("LoginServer.OnExit")
	if this.accountDb != nil {
		this.accountDb.(*mongodb.MongoDb).Disconnect()
	}
	if cache.GetRedis() != nil {
		cache.GetRedis().Close()
	}
}

// 初始化数据库
func (this *LoginServer) initDb() {
	// 使用mongodb来演示
	mongoDb := mongodb.NewMongoDb("mongodb://localhost:27017","testdb","account")
	mongoDb.SetAccountColumnNames("id", "name")
	if !mongoDb.Connect() {
		panic("connect db error")
	}
	this.accountDb = mongoDb
}

// 初始化redis缓存
func (this *LoginServer) initCache() {
	redisAddrs := []string{"10.0.75.2:6379"}
	cache.NewRedisClient(redisAddrs, "")
}

func (this *LoginServer) readConfig() {
	this.config = &LoginServerConfig{
		clientListenAddr: "127.0.0.1:10002",
		clientConnConfig: gnet.ConnectionConfig{
			SendPacketCacheCap: 8,
			SendBufferSize:     1024 * 10,
			RecvBufferSize:     1024 * 10,
			MaxPacketSize:      1024 * 10,
		},
	}
}

// 注册客户端消息回调
func (this *LoginServer) registerClientPacket(clientHandler *gnet.DefaultConnectionHandler) {
	clientHandler.Register(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), onHeartBeatReq, func() proto.Message {return &pb.HeartBeatReq{}})
	clientHandler.Register(gnet.PacketCommand(pb.CmdLogin_Cmd_LoginReq), onLoginReq, func() proto.Message {return &pb.LoginReq{}})
	clientHandler.Register(gnet.PacketCommand(pb.CmdLogin_Cmd_AccountReg), onAccountReg, func() proto.Message {return &pb.AccountReg{}})
}

// 客户端心跳回复
func onHeartBeatReq(connection gnet.Connection, packet *gnet.ProtoPacket) {
	req := packet.Message().(*pb.HeartBeatReq)
	connection.Send(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatRes), &pb.HeartBeatRes{
		RequestTimestamp: req.GetTimestamp(),
		ResponseTimestamp: uint64(time.Now().UnixNano()/int64(time.Microsecond)),
	})
}
