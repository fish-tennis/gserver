package internal

import (
	"context"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"io"
	"log/slog"
	"sync"
	"time"
)

type ListerConfig struct {
	Addr string `yaml:"Addr"`
}

type MongoConfig struct {
	Uri string `yaml:"Uri"`
	Db  string `yaml:"Db"`
}

type RedisConfig struct {
	Uri      []string `yaml:"Uri"`
	UserName string   `yaml:"UserName"`
	Password string   `yaml:"Password"`
	Cluster  bool     `yaml:"Cluster"`
}

type BaseServerConfig struct {
	// 服务器id
	ServerId int32        `yaml:"ServerId"`
	Client   ListerConfig `yaml:"Client"`
	Gate     ListerConfig `yaml:"Gate"`
	Server   ListerConfig `yaml:"Server"`
	Mongo    MongoConfig  `yaml:"Mongo"`
	Redis    RedisConfig  `yaml:"Redis"`
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
	ctx         context.Context
	wg          sync.WaitGroup
	serverHooks []gentity.ApplicationHook
}

func NewBaseServer(ctx context.Context, serverType string, configFile string) *BaseServer {
	return &BaseServer{
		ctx:        ctx,
		configFile: configFile,
		serverInfo: &pb.ServerInfo{
			ServerType: serverType,
		},
	}
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

// 加载配置文件
func (this *BaseServer) Init(ctx context.Context, configFile string) bool {
	slog.Info("BaseServer.Init")
	// 初始化id生成器
	util.InitIdGenerator(uint16(this.serverInfo.ServerId))
	network.InitCommandMappingFromFile("gen/message_command_mapping.json")
	this.serverList = NewServerList(this.serverInfo)
	this.updateInterval = time.Second
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

func (this *BaseServer) NewAdaptPacket(cmd PacketCommand, message proto.Message) Packet {
	if this.serverInfo.ServerType == ServerType_Gate {
		return network.NewGatePacket(0, cmd, message)
	} else {
		return NewProtoPacket(cmd, message)
	}
}

// 发消息给另一个服务器
func (this *BaseServer) SendToServer(serverId int32, cmd PacketCommand, message proto.Message) bool {
	return this.serverList.Send(serverId, cmd, message)
}
