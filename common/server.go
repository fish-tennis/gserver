package common

import (
	"context"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"google.golang.org/protobuf/proto"
	"io"
	"time"
)

// 服务器接口
type Server interface {
	// 初始化
	Init(ctx context.Context, configFile string) bool

	// 运行
	Run(ctx context.Context)

	// 定时更新
	OnUpdate(ctx context.Context, updateCount int64)

	// 退出
	Exit()
}

type BaseServerConfig struct {
	// 服务器id
	ServerId int32
	// 客户端监听地址
	ClientListenAddr string
	// 客户端连接配置
	ClientConnConfig gnet.ConnectionConfig
	// 服务器监听地址
	ServerListenAddr string
	// 服务器连接配置
	ServerConnConfig gnet.ConnectionConfig
	// mongodb地址
	MongoUri string
	// redis地址
	RedisUri []string
	RedisPassword string
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
	serverConnectorConfig gnet.ConnectionConfig
	// 默认的服务器连接接口
	defaultServerConnectorHandler gnet.ConnectionHandler
	// 默认的服务器之间的编解码
	defaultServerConnectorCodec *gnet.ProtoCodec
}

func (this *BaseServer) GetConfigFile() string {
	return this.configFile
}

func (this *BaseServer) GetServerId() int32 {
	return this.serverInfo.GetServerId()
}

func (this *BaseServer) GetServerInfo() *pb.ServerInfo {
	return this.serverInfo
}

func (this *BaseServer) GetServerList() *ServerList {
	return this.serverList
}

// 加载配置文件
func (this *BaseServer) Init(ctx context.Context, configFile string) bool {
	logger.Debug("BaseServer.Init")
	this.configFile = configFile
	this.serverInfo = new(pb.ServerInfo)
	this.serverList = NewServerList()
	this.serverList.localServerInfo = this.serverInfo
	this.serverList.serverConnectorFunc = this.DefaultServerConnectorFunc
	this.updateInterval = time.Second
	return true
}

// 运行
func (this *BaseServer) Run(ctx context.Context) {
	logger.Debug("BaseServer.Run")
	go func(ctx context.Context) {
		this.updateLoop(ctx)
	}(ctx)
}

func (this *BaseServer) OnUpdate(ctx context.Context, updateCount int64) {
	//gnet.LogDebug("BaseServer.OnUpdate")
	// 定时上传本地服务器的信息
	this.serverInfo.LastActiveTime = util.GetCurrentMS()
	this.GetServerList().Register(this.serverInfo)
	this.GetServerList().FindAndConnectServers(ctx)
}

func (this *BaseServer) Exit() {
	logger.Debug("BaseServer.Exit")
	// 网络关闭
	gnet.GetNetMgr().Shutdown(true)
	// 缓存关闭
	if cache.Get() != nil {
		if closer,ok := cache.Get().(io.Closer); ok {
			closer.Close()
			logger.Info("close redis")
		}
	}
}

// 定时更新接口
func (this *BaseServer) updateLoop(ctx context.Context) {
	logger.Debug("updateLoop begin")
	// 暂定更新间隔1秒
	updateTicker := time.NewTicker(this.updateInterval)
	defer func() {
		updateTicker.Stop()
		logger.Debug("updateLoop end")
	}()
	for {
		select {
		// 系统关闭通知
		case <-ctx.Done():
			logger.Debug("exitNotify")
			return
		case <-updateTicker.C:
			this.OnUpdate(ctx, this.updateCount)
			this.updateCount++
		}
	}
}

// 设置默认的服务器间的编解码和回调接口
func (this *BaseServer) SetDefaultServerConnectorConfig(config gnet.ConnectionConfig) {
	this.serverConnectorConfig = config
	this.defaultServerConnectorCodec = gnet.NewProtoCodec(nil)
	handler := gnet.NewDefaultConnectionHandler(this.defaultServerConnectorCodec)
	handler.SetOnDisconnectedFunc(func(connection gnet.Connection) {
		if connection.GetTag() == nil {
			return
		}
		serverId := connection.GetTag().(int32)
		this.serverList.OnServerConnectorDisconnect(serverId)
	})
	handler.RegisterHeartBeat(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), func() proto.Message {
		return &pb.HeartBeatReq{
			Timestamp: uint64(util.GetCurrentMS()),
		}
	})
	handler.Register(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatRes), func(connection gnet.Connection, packet *gnet.ProtoPacket) {
	}, func() proto.Message {
		return new(pb.HeartBeatRes)
	})
	this.defaultServerConnectorHandler = handler
}

// 默认的服务器连接接口
func (this *BaseServer) DefaultServerConnectorFunc(ctx context.Context, info *pb.ServerInfo) gnet.Connection {
	return gnet.GetNetMgr().NewConnector(ctx, info.GetServerListenAddr(), this.serverConnectorConfig,
		this.defaultServerConnectorCodec, this.defaultServerConnectorHandler, nil)
}

// 发消息给另一个服务器
func (this *BaseServer) SendToServer(serverId int32, cmd Cmd, message proto.Message) bool {
	return this.serverList.SendToServer(serverId, cmd, message)
}