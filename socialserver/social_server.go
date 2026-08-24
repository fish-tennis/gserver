package socialserver

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/social"
)

var (
	_ gentity.Application = (*SocialServer)(nil)
)

// 社交服:专门承载跨玩家协程实体(如公会),不处理玩家协程逻辑
// 公会等跨玩家的社交实体从game服务器迁移到独立的social服务器进程,
// 通过服务器间的消息路由与game服务器上的玩家协程交互
type SocialServer struct {
	*BaseServer
}

func NewSocialServer(ctx context.Context, configFile string, cfgDir string) *SocialServer {
	s := &SocialServer{
		BaseServer: NewBaseServer(ctx, ServerType_Social, configFile, cfgDir),
	}
	s.ReadConfig()
	return s
}

// 初始化
func (this *SocialServer) Init(ctx context.Context, configFile string) bool {
	if !this.BaseServer.Init(ctx, configFile) {
		return false
	}
	// 公会等社交模块的初始化接口
	this.AddServerHook(&social.Hook{})

	this.initDb()
	this.initCache()
	this.initNetwork()

	for _, hook := range this.BaseServer.GetServerHooks() {
		hook.OnApplicationInit(nil)
	}
	slog.Info("SocialServer.Init")
	return true
}

// 运行
func (this *SocialServer) Run(ctx context.Context) {
	this.BaseServer.Run(ctx)
	slog.Info("SocialServer.Run")
}

// 退出
func (this *SocialServer) Exit() {
	this.SetStatus(ServerStatus_Exit)
	// BaseServer.Exit内部会遍历ServerHooks调用OnApplicationExit,
	// 触发social.Hook的_guildMgr.StopAll,等待所有公会协程完成数据保存后再关闭基础设施
	this.BaseServer.Exit()
	slog.Info("SocialServer.Exit")
	if db.GetDbMgr() != nil {
		db.GetDbMgr().(*gentity.MongoDb).Disconnect()
	}
}

// 初始化数据库
func (this *SocialServer) initDb() {
	// 使用mongodb来演示
	mongoDb := gentity.NewMongoDb(this.GetConfig().Mongo.Uri, this.GetConfig().Mongo.Db)
	// 玩家数据库(跨玩家协程实体保存玩家的简要数据时使用)
	mongoDb.RegisterPlayerDb(db.PlayerDbName, true, db.UniqueIdName, db.PlayerAccountId, db.PlayerRegionId)
	// 公会数据库
	mongoDb.RegisterEntityDb(db.GuildDbName, true, db.UniqueIdName)
	if !mongoDb.Connect() {
		panic("connect db error")
	}
	db.SetDbMgr(mongoDb)
}

// 初始化redis缓存
func (this *SocialServer) initCache() {
	cache.NewRedis(this.GetConfig().Redis.Uri, this.GetConfig().Redis.UserName, this.GetConfig().Redis.Password, this.GetConfig().Redis.Cluster, this.GetConfig().Redis.DB)
	pong, err := cache.GetRedis().Ping(context.Background()).Result()
	if err != nil || pong == "" {
		panic(fmt.Sprintf("redis connect error,uri:%v err:%v pong:%v", this.GetConfig().Redis.Uri, err, pong))
	}
}

func (this *SocialServer) initNetwork() {
	this.GetServerList().SetCache(cache.Get())
	// 注册业务层的消息回调
	serverHandlers := []*DefaultConnectionHandler{
		this.GetServerList().GetServerConnectionHandler(),
		this.GetServerList().GetServerListenerHandler(),
	}
	for _, serverHandler := range serverHandlers {
		// 其他模块注册服务器之间的消息回调
		for _, hook := range this.GetServerHooks() {
			hook.OnRegisterServerHandler(serverHandler)
		}
	}
	this.GetServerList().SetFetchAndConnectServerTypes(ServerType_Social, ServerType_Game)
	// 通用的服务器间的监听
	if this.GetServerList().StartListen(this.GetContext(), this.GetConfig().Server.Addr) == nil {
		panic("listen server failed")
	}
}
