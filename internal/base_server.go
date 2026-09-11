package internal

import (
	"context"
	"fmt"
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
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

type ListerConfig struct {
	Addr string `yaml:"Addr"` // 监听地址
	Url  string `yaml:"Url"`  // 开放给客户端的的连接地址
}

type MongoConfig struct {
	Uri string `yaml:"Uri"`
	Db  string `yaml:"Db"`
	// OpTimeoutSec 单次CRUD操作超时时间(秒),0=使用框架默认值(60秒)
	// 默认值针对极低概率的"静默卡死"兜底,宁大勿小防误伤排队/负载抖动;
	// 追求更快的故障隔离(容忍误伤)时可调小,如10~20
	OpTimeoutSec int `yaml:"OpTimeoutSec"`
	// MaxPoolSize MongoDB连接池上限,0=驱动默认值(100)
	// 多个game进程共用同一MongoDB实例时,所有进程连接池上限之和不应超过
	// 实例的maxConnections,建议按单进程并发需求收紧(如每game进程32)
	MaxPoolSize uint64 `yaml:"MaxPoolSize"`
	// MinPoolSize 连接池预热下限,0=驱动默认值(0)
	// 设8~16可避免停服/开服突发时从零建连(每次建连含握手/认证,约几十毫秒)
	MinPoolSize uint64 `yaml:"MinPoolSize"`
	// MaxConnIdleTimeSec 连接最大空闲时间(秒),0=驱动默认值(不回收)
	// 设300~600可让白天高峰建的连接在夜间低谷回收,防止长期占用实例连接数
	MaxConnIdleTimeSec int `yaml:"MaxConnIdleTimeSec"`
	// ConnectTimeoutSec 建连超时(秒),0=驱动默认值(30)
	// 缩短(如5)可让Mongo不可达时进程启动快速失败,而非挂默认30秒
	ConnectTimeoutSec int `yaml:"ConnectTimeoutSec"`
	// WriteConcern 写关注级别:"1"或"majority",空=沿用实例默认
	// 存档写已有Redis兜底+云实例双机热备,建议"1";majority会显著降低写吞吐
	WriteConcern string `yaml:"WriteConcern"`
	// Compressors 网络压缩算法,如["snappy"]或["zstd"],空=不压缩
	// 注意:大文档+zstd的CPU开销可能反噬吞吐(腾讯云实测500KB文档zstd比snappy慢36%),带宽紧张时才建议开启并实测
	Compressors []string `yaml:"Compressors"`
}

// BuildMongoClientOptions 从Mongo配置构造客户端options
// 仅映射显式配置项(非零值),未配置的沿用uri参数或驱动默认值
// NOTE: AppName由BaseServer.NewMongoDb自动设置为servertype_serverId,此处无需处理
func BuildMongoClientOptions(cfg *MongoConfig) *options.ClientOptions {
	clientOpts := options.Client()
	if cfg.MaxPoolSize > 0 {
		clientOpts.SetMaxPoolSize(cfg.MaxPoolSize)
	}
	if cfg.MinPoolSize > 0 {
		clientOpts.SetMinPoolSize(cfg.MinPoolSize)
	}
	if cfg.MaxConnIdleTimeSec > 0 {
		clientOpts.SetMaxConnIdleTime(time.Duration(cfg.MaxConnIdleTimeSec) * time.Second)
	}
	if cfg.ConnectTimeoutSec > 0 {
		clientOpts.SetConnectTimeout(time.Duration(cfg.ConnectTimeoutSec) * time.Second)
	}
	switch cfg.WriteConcern {
	case "majority":
		clientOpts.SetWriteConcern(writeconcern.Majority())
	case "1":
		clientOpts.SetWriteConcern(writeconcern.W1())
	}
	if len(cfg.Compressors) > 0 {
		clientOpts.SetCompressors(cfg.Compressors)
	}
	return clientOpts
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
	status     atomic.Int32
	ctx        context.Context
	ctxCancel  context.CancelFunc
	wg         sync.WaitGroup
	serverHooks []gentity.ApplicationHook
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
			// StartupTime记录进程启动时刻:NewBaseServer在所有协程启动之前执行
			// (每秒读serverInfo的updateLoop由Run()才启动),此处直接写serverInfo无并发风险,
			// 之后由RegisterLocalServerInfo每秒随心跳上报
			StartupTime: time.Now().Unix(),
			// GitVersion在包init时已确定最终值(ldflags注入优先,其次构建时VCS嵌入),
			// 同包直接引用包变量,无需等待配置
			GitVersion: GitVersion,
		},
	}
	// 创建可取消的 context,确保 Exit() 能主动触发 updateLoop 退出
	s.ctx, s.ctxCancel = context.WithCancel(ctx)
	// 初始状态为 Init
	s.status.Store(int32(ServerStatus_Init))
	return s
}

func (this *BaseServer) GetConfig() *BaseServerConfig {
	return this.config
}

// NewMongoDb 根据配置创建MongoDb实例(各server的initDb统一入口)
// 客户端连接参数(连接池/超时/写关注/压缩等)已在内部按MongoConfig映射,
// AppName自动设置为servertype_serverId(如game_101),无需手动配置——
// 在Mongo侧的currentOp/连接列表中可直接定位连接来自哪个进程
// 后续RegisterXxxDb -> Connect的标准流程不变
func (this *BaseServer) NewMongoDb() *gentity.MongoDb {
	clientOpts := BuildMongoClientOptions(&this.GetConfig().Mongo).
		SetAppName(fmt.Sprintf("%v_%v", this.serverInfo.ServerType, this.GetId()))
	return gentity.NewMongoDb(this.GetConfig().Mongo.Uri, this.GetConfig().Mongo.Db).
		SetClientOptions(clientOpts)
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
	// 应用MongoDB单次操作超时配置(在所有server的initDb之前执行,全程生效)
	// >0才覆盖,0保持框架默认值;GateServer等不连Mongo的进程设置了也无害
	if this.config.Mongo.OpTimeoutSec > 0 {
		gentity.SetMongoOpTimeout(time.Duration(this.config.Mongo.OpTimeoutSec) * time.Second)
	}
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
	go func() {
		defer this.wg.Done()
		// 使用 this.ctx(可取消的子 context),确保 Exit() 的 ctxCancel 能触发 updateLoop 退出
		// 不能用外部传入的 ctx(父 context),否则 Exit 单独调用时无法取消 updateLoop
		this.updateLoop(this.ctx)
	}()
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
	// 取消 context,确保 updateLoop 协程退出,不再依赖外部调用方取消 context
	if this.ctxCancel != nil {
		this.ctxCancel()
	}
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
