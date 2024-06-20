package loginserver

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"log/slog"
	"os"
)

var (
	_ gentity.Application = (*LoginServer)(nil)
	// singleton
	_loginServer *LoginServer
)

// 登录服
type LoginServer struct {
	BaseServer
	config *LoginServerConfig
	// 网关服务器listener
	gateListener Listener
	// 账号数据接口
	accountDb gentity.EntityDb
}

// 登录服配置
type LoginServerConfig struct {
	BaseServerConfig
}

// 账号db接口
func (this *LoginServer) GetAccountDb() gentity.EntityDb {
	return this.accountDb
}

// 初始化
func (this *LoginServer) Init(ctx context.Context, configFile string) bool {
	_loginServer = this
	if !this.BaseServer.Init(ctx, configFile) {
		return false
	}
	this.readConfig()
	this.initDb()
	this.initCache()
	this.GetServerList().SetCache(cache.Get())
	netMgr := GetNetMgr()
	// NOTE: 实际项目中,监听客户端和监听网关,二选一即可
	// 这里为了演示,同时提供客户端直连和网关两种模式
	clientCodec := NewProtoCodec(nil)
	clientHandler := NewDefaultConnectionHandler(clientCodec)
	this.registerClientPacket(clientHandler)
	this.config.ClientConnConfig.Codec = clientCodec
	this.config.ClientConnConfig.Handler = clientHandler
	listenerConfig := &ListenerConfig{
		AcceptConfig: this.config.ClientConnConfig,
	}
	if netMgr.NewListener(ctx, this.config.ClientListenAddr, listenerConfig) == nil {
		panic("listen failed")
		return false
	}

	// NOTE: 实际项目中,监听客户端和监听网关,二选一即可
	// 这里为了演示,同时提供客户端直连和网关两种模式
	// 服务器的codec和handler
	serverCodec := NewGateCodec(nil)
	serverHandler := NewDefaultConnectionHandler(serverCodec)
	this.registerServerPacket(serverHandler)
	gateListenerConfig := &ListenerConfig{
		AcceptConfig: this.config.ServerConnConfig,
	}
	gateListenerConfig.AcceptConfig.Codec = serverCodec
	gateListenerConfig.AcceptConfig.Handler = serverHandler
	this.gateListener = netMgr.NewListener(ctx, this.config.GateListenAddr, gateListenerConfig)
	if this.gateListener == nil {
		panic("listen gate failed")
		return false
	}

	this.BaseServer.GetServerList().SetFetchServerTypes(ServerType_Game)

	slog.Info("LoginServer.Init")
	return true
}

// 运行
func (this *LoginServer) Run(ctx context.Context) {
	this.BaseServer.Run(ctx)
	slog.Info("LoginServer.Run")
}

// 退出
func (this *LoginServer) Exit() {
	this.BaseServer.Exit()
	slog.Info("LoginServer.Exit")
	if db.GetDbMgr() != nil {
		db.GetDbMgr().(*gentity.MongoDb).Disconnect()
	}
}

// 读取配置文件
func (this *LoginServer) readConfig() {
	fileData, err := os.ReadFile(this.GetConfigFile())
	if err != nil {
		panic("read config file err")
	}
	this.config = new(LoginServerConfig)
	err = json.Unmarshal(fileData, this.config)
	if err != nil {
		panic("decode config file err")
	}
	slog.Debug("readConfig", "config", this.config)
	this.BaseServer.GetServerInfo().ServerId = this.config.ServerId
	this.BaseServer.GetServerInfo().ServerType = ServerType_Login
	// NOTE: 实际项目中,监听客户端和监听网关,二选一即可
	// 这里为了演示,同时提供客户端直连和网关两种模式
	this.BaseServer.GetServerInfo().ClientListenAddr = this.config.ClientListenAddr
	this.BaseServer.GetServerInfo().GateListenAddr = this.config.GateListenAddr
}

// 初始化数据库
func (this *LoginServer) initDb() {
	// 使用mongodb来演示
	mongoDb := gentity.NewMongoDb(this.config.MongoUri, this.config.MongoDbName)
	// 账号数据库
	this.accountDb = mongoDb.RegisterEntityDb(db.AccountDbName, true, db.UniqueIdName)
	// kv数据库
	mongoDb.RegisterKvDb(db.GlobalDbName, true, db.GlobalDbKeyName, db.GlobalDbValueName)
	if !mongoDb.Connect() {
		panic("connect db error")
	}
	this.accountDb.(*gentity.MongoCollection).CreateIndex(db.AccountName, true)
	db.SetDbMgr(mongoDb)
}

// 初始化redis缓存
func (this *LoginServer) initCache() {
	cache.NewRedis(this.config.RedisUri, this.config.RedisUsername, this.config.RedisPassword, this.config.RedisCluster)
	pong, err := cache.GetRedis().Ping(context.Background()).Result()
	if err != nil || pong == "" {
		panic("redis connect error")
	}
}

func (this *LoginServer) getAccountData(accountName string, accountData *pb.Account) error {
	mongoCol := this.GetAccountDb().(*gentity.MongoCollection)
	col := mongoCol.GetCollection()
	result := col.FindOne(context.Background(), bson.D{{db.AccountName, accountName}})
	if result == nil || errors.Is(result.Err(), mongo.ErrNoDocuments) {
		return nil
	}
	err := result.Decode(accountData)
	if err != nil {
		return err
	}
	// Q:_id为什么不会赋值?
	// A:因为protobuf自动生成的struct tag,无法适配mongodb的_id字段
	// 解决方案: 使用工具生成自定义的struct tag,如github.com/favadi/protoc-go-inject-tag
	// 如果能生成下面这种struct tag,就可以直接把mongodb的_id的值赋值到accountData.XId了
	// XId int64 `protobuf:"varint,1,opt,name=_id,json=Id,proto3" json:"_id,omitempty" bson:"_id"`
	if accountData.XId == 0 {
		raw, err := result.Raw()
		if err != nil {
			return err
		}
		idValue, err := raw.LookupErr("_id")
		if err != nil {
			return err
		}
		accountData.XId = idValue.Int64()
	}
	return nil
}

// 注册客户端消息回调
func (this *LoginServer) registerClientPacket(clientHandler PacketHandlerRegister) {
	clientHandler.Register(PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), onHeartBeatReq, new(pb.HeartBeatReq))
	clientHandler.Register(PacketCommand(pb.CmdLogin_Cmd_LoginReq), onLoginReq, new(pb.LoginReq))
	clientHandler.Register(PacketCommand(pb.CmdLogin_Cmd_AccountReg), onAccountReg, new(pb.AccountReg))
}

// 心跳回复
func onHeartBeatReq(connection Connection, packet Packet) {
	req := packet.Message().(*pb.HeartBeatReq)
	SendPacketAdapt(connection, packet, PacketCommand(pb.CmdInner_Cmd_HeartBeatRes), &pb.HeartBeatRes{
		RequestTimestamp:  req.GetTimestamp(),
		ResponseTimestamp: util.GetCurrentMS(),
	})
}

// 注册服务器消息回调
func (this *LoginServer) registerServerPacket(serverHandler PacketHandlerRegister) {
	this.registerClientPacket(serverHandler)
}
