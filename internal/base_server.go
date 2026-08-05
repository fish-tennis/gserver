package internal

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
	gserverutil "github.com/fish-tennis/gserver/util"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
)

type ListerConfig struct {
	Addr string `yaml:"Addr"` // 监听地址
	Url  string `yaml:"Url"`  // 开放给客户端的的连接地址
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
	DB       int      `yaml:"DB"`
}

type BaseServerConfig struct {
	// 服务器id
	ServerId int32 `yaml:"ServerId"`
	// 是否开启测试命令(仅测试环境开启,防止正式服作弊)
	IsOpenTestCommand bool         `yaml:"IsOpenTestCommand"`
	Client            ListerConfig `yaml:"Client"`
	Gate              ListerConfig `yaml:"Gate"`
	Server            ListerConfig `yaml:"Server"`
	// WebSocket客户端监听(仅GateServer使用,其他服务器不处理)
	WsClient     ListerConfig `yaml:"WsClient"`
	Mongo        MongoConfig  `yaml:"Mongo"`
	Redis        RedisConfig  `yaml:"Redis"`
	AlertWebhook string       `yaml:"AlertWebhook"` // 接收告警信息的webhook地址
}

// 服务器运行状态
type ServerStatus int32

const (
	ServerStatus_Init    ServerStatus = 0 // 初始化中
	ServerStatus_Running ServerStatus = 1 // 运行中
	ServerStatus_Exit    ServerStatus = 2 // 正在退出
)

// 服务器基础流程
type BaseServer struct {
	// 配置
	config *BaseServerConfig
	// 配置文件
	configFile string
	// 配置数据目录
	cfgDir string
	// 自己的服务器信息
	serverInfo *pb.ServerInfo
	// 服务器列表
	serverList *ServerList
	// 定时更新间隔
	updateInterval time.Duration
	// 更新次数
	updateCount int64
	// 告警webhook地址
	alertWebhook string
	// 服务器运行状态
	status atomic.Int32
	ctx          context.Context
	wg           sync.WaitGroup
	serverHooks  []gentity.ApplicationHook
}

func NewBaseServer(ctx context.Context, serverType string, configFile string, cfgDir string) *BaseServer {
	cfgDir = filepath.ToSlash(cfgDir)
	if strings.LastIndexByte(cfgDir, '/') != len(cfgDir)-1 {
		cfgDir += string('/')
	}
	s := &BaseServer{
		config:     new(BaseServerConfig),
		ctx:        ctx,
		configFile: configFile,
		cfgDir:     cfgDir,
		serverInfo: &pb.ServerInfo{
			ServerType: serverType,
		},
	}
	// 初始状态为 Init
	s.status.Store(int32(ServerStatus_Init))
	return s
}

func (this *BaseServer) GetConfig() *BaseServerConfig {
	return this.config
}

func (this *BaseServer) GetConfigFile() string {
	return this.configFile
}

func (this *BaseServer) GetCfgDir() string {
	return this.cfgDir
}

// 读取配置文件(统一逻辑,子类无需重复实现)
func (this *BaseServer) ReadConfig() {
	fileData, err := os.ReadFile(this.configFile)
	if err != nil {
		panic("read config file err: " + err.Error())
	}
	err = yaml.Unmarshal(fileData, this.config)
	if err != nil {
		panic("decode config file err: " + err.Error())
	}
	slog.Debug("ReadConfig", "config", this.config)
	this.serverInfo.ServerId = this.config.ServerId
	this.serverInfo.ClientListenAddr = this.config.Client.Addr
	this.serverInfo.GateListenAddr = this.config.Gate.Addr
	this.serverInfo.ServerListenAddr = this.config.Server.Addr
	if this.config.WsClient.Url != "" {
		this.serverInfo.WsClientListenAddr = this.config.WsClient.Url
	} else if this.config.WsClient.Addr != "" {
		this.serverInfo.WsClientListenAddr = "ws://" + this.config.WsClient.Addr + "/ws"
	} else {
		this.serverInfo.WsClientListenAddr = ""
	}
	this.SetAlertWebhook(this.config.AlertWebhook)
}

func (this *BaseServer) GetId() int32 {
	return this.serverInfo.GetServerId()
}

func (this *BaseServer) SetAlertWebhook(webhook string) {
	this.alertWebhook = webhook
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

// 服务器是否处于运行状态
func (this *BaseServer) IsRunning() bool {
	return ServerStatus(this.status.Load()) == ServerStatus_Running
}

// 获取服务器当前状态
func (this *BaseServer) GetStatus() ServerStatus {
	return ServerStatus(this.status.Load())
}

func (this *BaseServer) SetStatus(status ServerStatus) {
	this.status.Store(int32(status))
}

// 加载配置文件
func (this *BaseServer) Init(ctx context.Context, configFile string) bool {
	slog.Info("BaseServer.Init")
	// 初始化id生成器
	util.InitIdGenerator(uint16(this.serverInfo.ServerId))
	network.InitCommandMappingFromFile(this.GetCfgDir() + "message_command_mapping.json")
	this.serverList = NewServerList(this.serverInfo)
	this.updateInterval = time.Second
	// 初始化告警模块
	if this.alertWebhook != "" {
		workDir, _ := os.Getwd()
		InitAlert(this.alertWebhook, this.GetId(), this.serverInfo.ServerType,
			gserverutil.GetLocalIP(), workDir, BuildTime, BuildType, GitVersion)
	}
	return true
}

// 运行
func (this *BaseServer) Run(ctx context.Context) {
	this.status.Store(int32(ServerStatus_Running))
	slog.Info("BaseServer.Run")
	this.wg.Add(1)
	go func(ctx context.Context) {
		defer this.wg.Done()
		this.updateLoop(ctx)
	}(ctx)
}

func (this *BaseServer) OnUpdate(ctx context.Context, updateCount int64) {
	// 定时上传本地服务器的信息
	this.serverInfo.LastActiveTime = util.GetCurrentMS()
	this.GetServerList().RegisterLocalServerInfo()
	this.GetServerList().FindAndConnectServers(ctx)
}

func (this *BaseServer) Exit() {
	this.status.Store(int32(ServerStatus_Exit))
	slog.Info("BaseServer.Exit")
	for _, hook := range this.serverHooks {
		hook.OnApplicationExit()
	}
	// 服务器管理的协程关闭
	slog.Info("wait server goroutine close")
	this.wg.Wait()
	slog.Info("all server goroutine closed")
	// 网络关闭
	slog.Info("wait net goroutine close")
	GetNetMgr().Shutdown(true)
	slog.Info("all net goroutine closed")
	// 缓存关闭
	if cache.GetRedis() != nil {
		if closer, ok := cache.GetRedis().(io.Closer); ok {
			slog.Info("wait redis close")
			closer.Close()
			slog.Info("redis closed")
		}
	}
}

// 定时更新接口
func (this *BaseServer) updateLoop(ctx context.Context) {
	slog.Info("updateLoop begin")
	// 暂定更新间隔1秒
	updateTicker := time.NewTicker(this.updateInterval)
	defer func() {
		updateTicker.Stop()
		slog.Info("updateLoop end")
	}()
	for {
		select {
		// 系统关闭通知
		case <-ctx.Done():
			slog.Info("exitNotify")
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
