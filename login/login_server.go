package login

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
	common.BaseServerConfig
}

func (this *LoginServer) GetAccountDb() db.AccountDb {
	return this.accountDb
}

func (this *LoginServer) Init(ctx context.Context, configFile string) bool {
	loginServer = this
	if !this.BaseServer.Init(ctx, configFile) {
		return false
	}
	this.readConfig()
	this.initDb()
	this.initCache()
	netMgr := gnet.GetNetMgr()
	clientCodec := gnet.NewProtoCodec(nil)
	clientHandler := gnet.NewDefaultConnectionHandler(clientCodec)
	this.registerClientPacket(clientHandler)
	if netMgr.NewListener(ctx, this.config.ClientListenAddr, this.config.ClientConnConfig, clientCodec, clientHandler, nil) == nil {
		panic("listen failed")
		return false
	}

	// 连接其他服务器
	this.BaseServer.SetDefaultServerConnectorConfig(this.config.ServerConnConfig)
	this.BaseServer.GetServerList().SetFetchAndConnectServerTypes("game")
	gnet.LogDebug("LoginServer.Init")
	return true
}

func (this *LoginServer) Run(ctx context.Context) {
	this.BaseServer.Run(ctx)
	gnet.LogDebug("LoginServer.Run")
}

func (this *LoginServer) Exit() {
	this.BaseServer.Exit()
	gnet.LogDebug("LoginServer.Exit")
	if this.accountDb != nil {
		this.accountDb.(*mongodb.MongoDb).Disconnect()
	}
}

func (this *LoginServer) readConfig() {
	fileData,err := os.ReadFile(this.GetConfigFile())
	if err != nil {
		panic("read config file err")
	}
	this.config = new(LoginServerConfig)
	err = json.Unmarshal(fileData, this.config)
	if err != nil {
		panic("decode config file err")
	}
	gnet.LogDebug("%v", this.config)
	this.BaseServer.GetServerInfo().ServerId = this.config.ServerId
	this.BaseServer.GetServerInfo().ServerType = "login"
	this.BaseServer.GetServerInfo().ClientListenAddr = this.config.ClientListenAddr
}

// 初始化数据库
func (this *LoginServer) initDb() {
	// 使用mongodb来演示
	mongoDb := mongodb.NewMongoDb(this.config.MongoUri,"testdb","account")
	mongoDb.SetAccountColumnNames("id", "name")
	if !mongoDb.Connect() {
		panic("connect db error")
	}
	this.accountDb = mongoDb
}

// 初始化redis缓存
func (this *LoginServer) initCache() {
	cache.NewRedisClient(this.config.RedisUri, this.config.RedisPassword)
	pong,err := cache.GetRedis().Ping(context.TODO()).Result()
	if err != nil || pong == "" {
		panic("redis connect error")
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

// 发消息给另一个服务器
func (this *LoginServer) SendToServer(serverId int32, cmd Cmd, message proto.Message) bool {
	return this.GetServerList().SendToServer(serverId, cmd, message)
}
