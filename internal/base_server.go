package internal

import (
	"context"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"io"
	"sync"
	"time"
)

type BaseServerConfig struct {
	// 服务器id
	ServerId int32
	// 客户端监听地址
	ClientListenAddr string
	// 客户端连接配置
	ClientConnConfig ConnectionConfig
	// 网关监听地址
	GateListenAddr string
	// 其他服务器监听地址
	ServerListenAddr string
	// 服务器连接配置
	ServerConnConfig ConnectionConfig
	// mongodb地址
	MongoUri string
	// mongodb db name
	MongoDbName string
	// redis地址
	RedisUri      []string
	RedisPassword string
	// 是否使用redis集群模式
	RedisCluster bool
}

// 服务器基础流程
type BaseServer struct {
	// 配置文件
	configFile string
	// 自己的服务器信息
	serverInfo *pb.ServerInfo
	// 服务器列表
	serverList *ServerList
	// 定时更新间隔
	updateInterval time.Duration
	// 更新次数
	updateCount int64
	// 服务器连接配置
	serverConnectorConfig ConnectionConfig
	// 默认的服务器连接接口
	defaultServerConnectorHandler ConnectionHandler
	// 默认的服务器之间的编解码
	defaultServerConnectorCodec Codec
	ctx                         context.Context
	wg                          sync.WaitGroup
	serverHooks                 []gentity.ApplicationHook
}

func (this *BaseServer) GetConfigFile() string {
	return this.configFile
}

func (this *BaseServer) GetId() int32 {
	return this.serverInfo.GetServerId()
}

func (this *BaseServer) GetContext() context.Context {
	return this.ctx
}

func (this *BaseServer) GetWaitGroup() *sync.WaitGroup {
	return &this.wg
}

func (this *BaseServer) GetServerInfo() *pb.ServerInfo {
	return this.serverInfo
}

func (this *BaseServer) GetServerList() *ServerList {
	return this.serverList
}

func (this *BaseServer) AddServerHook(hooks ...gentity.ApplicationHook) {
	this.serverHooks = append(this.serverHooks, hooks...)
}

func (this *BaseServer) GetServerHooks() []gentity.ApplicationHook {
	return this.serverHooks
}

func (this *BaseServer) GetDefaultServerConnectorHandler() ConnectionHandler {
	return this.defaultServerConnectorHandler
}

func (this *BaseServer) GetDefaultServerConnectorCodec() Codec {
	return this.defaultServerConnectorCodec
}

// 加载配置文件
func (this *BaseServer) Init(ctx context.Context, configFile string) bool {
	logger.Info("BaseServer.Init")
	this.configFile = configFile
	this.serverInfo = new(pb.ServerInfo)
	this.serverList = NewServerList()
	this.serverList.SetLocalServerInfo(this.serverInfo)
	this.serverList.SetServerConnectorFunc(this.DefaultServerConnectorFunc)
	this.updateInterval = time.Second
	// 初始化id生成器
	util.InitIdGenerator(uint16(this.serverInfo.ServerId))
	this.ctx = ctx
	return true
}

// 运行
func (this *BaseServer) Run(ctx context.Context) {
	logger.Info("BaseServer.Run")
	go func(ctx context.Context) {
		this.updateLoop(ctx)
	}(ctx)
}

func (this *BaseServer) OnUpdate(ctx context.Context, updateCount int64) {
	//LogDebug("BaseServer.OnUpdate")
	// 定时上传本地服务器的信息
	this.serverInfo.LastActiveTime = util.GetCurrentMS()
	this.GetServerList().RegisterLocalServerInfo()
	this.GetServerList().FindAndConnectServers(ctx)
}

func (this *BaseServer) Exit() {
	logger.Info("BaseServer.Exit")
	for _, hook := range this.serverHooks {
		hook.OnApplicationExit()
	}
	// 服务器管理的协程关闭
	logger.Info("wait server goroutine close")
	this.wg.Wait()
	logger.Info("all server goroutine closed")
	// 网络关闭
	logger.Info("wait net goroutine close")
	GetNetMgr().Shutdown(true)
	logger.Info("all net goroutine closed")
	// 缓存关闭
	if cache.GetRedis() != nil {
		if closer, ok := cache.GetRedis().(io.Closer); ok {
			logger.Info("wait redis close")
			closer.Close()
			logger.Info("redis closed")
		}
	}
}

// 定时更新接口
func (this *BaseServer) updateLoop(ctx context.Context) {
	logger.Info("updateLoop begin")
	// 暂定更新间隔1秒
	updateTicker := time.NewTicker(this.updateInterval)
	defer func() {
		updateTicker.Stop()
		logger.Info("updateLoop end")
	}()
	for {
		select {
		// 系统关闭通知
		case <-ctx.Done():
			logger.Info("exitNotify")
			return
		case <-updateTicker.C:
			this.OnUpdate(ctx, this.updateCount)
			this.updateCount++
		}
	}
}

// 设置默认的服务器间的编解码和回调接口
func (this *BaseServer) SetDefaultServerConnectorConfig(config ConnectionConfig, defaultServerConnectorCodec Codec) {
	this.serverConnectorConfig = config
	this.defaultServerConnectorCodec = defaultServerConnectorCodec
	handler := NewDefaultConnectionHandler(this.defaultServerConnectorCodec)
	handler.SetOnDisconnectedFunc(func(connection Connection) {
		if connection.GetTag() == nil {
			return
		}
		serverId := connection.GetTag().(int32)
		this.serverList.OnServerConnectorDisconnect(serverId)
	})
	handler.RegisterHeartBeat(func() Packet {
		return NewProtoPacket(PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), &pb.HeartBeatReq{
			Timestamp: uint64(util.GetCurrentMS()),
		})
	})
	handler.Register(PacketCommand(pb.CmdInner_Cmd_HeartBeatRes), func(connection Connection, packet Packet) {
	}, new(pb.HeartBeatRes))
	this.defaultServerConnectorHandler = handler
}

// 默认的服务器连接接口
func (this *BaseServer) DefaultServerConnectorFunc(ctx context.Context, info gentity.ServerInfo) Connection {
	serverInfo := info.(*pb.ServerInfo)
	return GetNetMgr().NewConnector(ctx, serverInfo.GetServerListenAddr(), &this.serverConnectorConfig,
		this.defaultServerConnectorCodec, this.defaultServerConnectorHandler, nil)
}

// 发消息给另一个服务器
func (this *BaseServer) SendToServer(serverId int32, cmd PacketCommand, message proto.Message) bool {
	return this.serverList.Send(serverId, cmd, message)
}
