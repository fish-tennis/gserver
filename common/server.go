package common

import (
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"google.golang.org/protobuf/proto"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// 服务器接口
type Server interface {
	Init(configFile string) bool
	Run()
	OnUpdate(updateCount int64)
	OnExit()
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
	// 退出通知
	exitNotify chan os.Signal
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

// 加载配置,网络初始化等
func (this *BaseServer) Init(configFile string) bool {
	gnet.LogDebug("BaseServer.Init")
	this.configFile = configFile
	this.serverInfo = new(pb.ServerInfo)
	this.serverList = NewServerList()
	this.serverList.localServerInfo = this.serverInfo
	this.serverList.serverConnectorFunc = this.DefaultServerConnectorFunc
	this.updateInterval = time.Second
	this.exitNotify = make(chan os.Signal, 1)
	return true
}

// 运行
func (this *BaseServer) Run() {
	gnet.LogDebug("BaseServer.Run")
	go func() {
		this.updateLoop()
	}()
}

func (this *BaseServer) OnUpdate(updateCount int64) {
	//gnet.LogDebug("BaseServer.OnUpdate")
	// 定时上传本地服务器的信息
	this.serverInfo.LastActiveTime = util.GetCurrentMS()
	this.GetServerList().UploadServerInfo(this.serverInfo)
	this.GetServerList().FetchAndConnectServers()
}

func (this *BaseServer) OnExit() {
	gnet.LogDebug("BaseServer.OnExit")
}

// 等待系统关闭信号
func (this *BaseServer) WaitExit() {
	gnet.LogDebug("BaseServer.WaitExit")
	signal.Notify(this.exitNotify, os.Interrupt, os.Kill, syscall.SIGTERM)
	// TODO: windows系统上,加一个控制台输入,以方便调试
	select {
	case <-this.exitNotify:
		gnet.LogDebug("exitNotify")
		break
	}
	// 业务关闭处理
	this.OnExit()
	// 网络关闭
	gnet.GetNetMgr().Shutdown(true)
	// 缓存关闭
	if cache.GetRedis() != nil {
		cache.GetRedis().Close()
	}
	gnet.LogDebug("Exit")
}

// 定时更新接口
func (this *BaseServer) updateLoop() {
	gnet.LogDebug("updateLoop begin")
	// 暂定更新间隔1秒
	updateTimer := time.NewTimer(this.updateInterval)
	defer updateTimer.Stop()
	for {
		select {
		case <-this.exitNotify:
			gnet.LogDebug("exitNotify")
			break
		case <-updateTimer.C:
			this.OnUpdate(this.updateCount)
			this.updateCount++
			updateTimer.Reset(this.updateInterval)
		}
	}
	gnet.LogDebug("updateLoop end")
}

// 设置默认的服务器间的编解码和回调接口
func (this *BaseServer) SetDefaultServerConnectorConfig(config gnet.ConnectionConfig) {
	this.serverConnectorConfig = config
	this.defaultServerConnectorCodec = gnet.NewProtoCodec(nil)
	handler := gnet.NewDefaultConnectionHandler(this.defaultServerConnectorCodec)
	handler.SetOnDisconnectedFunc(func(connection gnet.Connection) {
		serverId := connection.GetTag().(int32)
		this.serverList.OnServerConnectorDisconnect(serverId)
	})
	handler.RegisterHeartBeat(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), func() proto.Message {
		return &pb.HeartBeatReq{
			Timestamp: uint64(util.GetCurrentMS()),
		}
	})
	this.defaultServerConnectorHandler = handler
}

// 默认的服务器连接接口
func (this *BaseServer) DefaultServerConnectorFunc(info *pb.ServerInfo) gnet.Connection {
	return gnet.NewTcpConnector(this.serverConnectorConfig, this.defaultServerConnectorCodec, this.defaultServerConnectorHandler)
}