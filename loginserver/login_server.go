package loginserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/db"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var (
	_ gentity.Application = (*LoginServer)(nil)
	// singleton
	_loginServer *LoginServer
)

// 登录服
type LoginServer struct {
	*BaseServer
	// 网关服务器listener
	gateListener Listener
	// 账号数据接口
	accountDb gentity.EntityDb
}

func NewLoginServer(ctx context.Context, configFile string, cfgDir string) *LoginServer {
	s := &LoginServer{
		BaseServer: NewBaseServer(ctx, ServerType_Login, configFile, cfgDir),
	}
	s.ReadConfig()
	return s
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
	// 加载配置数据,用于读取GlobalCfg等策划配置
	if err := cfg.Load(this.GetCfgDir(), nil); err != nil {
		panic(fmt.Sprintf("LoginServer loadCfgs:%v", err))
	}
	// 启动加载成功后建立md5快照,首次热更即走增量路径(见cfg/reload.go)
	cfg.InitMd5Snapshot(this.GetCfgDir())
	this.initDb()
	this.initCache()
	// 订阅热更配置通知,收到通知后按md5快照diff选择性重载本进程配置表
	cache.SubscribeReloadConfig(this.GetContext(), func() {
		if err := cfg.Reload(this.GetCfgDir()); err != nil {
			slog.Error("LoginServer reload config failed", "error", err)
		} else {
			slog.Info("LoginServer config reloaded")
		}
	})
	this.initRegions()
	this.initNetwork()
	// 初始化DB操作协程池,将登录/注册等含DB查询的请求从收包goroutine卸载
	InitDbWorkerPool()
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
	this.SetStatus(ServerStatus_Exit)
	// 先关闭DB操作协程池,等待在途登录请求(会访问Redis)全部完成
	ShutdownDbWorkerPool()
	// 再关闭网络、Redis 等基础设施
	this.BaseServer.Exit()
	slog.Info("LoginServer.Exit")
	if db.GetDbMgr() != nil {
		db.GetDbMgr().(*gentity.MongoDb).Disconnect()
	}
}

// 初始化数据库
func (this *LoginServer) initDb() {
	// 使用mongodb来演示
	// 顺序要求:必须先Register再Connect——框架Connect会回填各集合的client/db句柄,
	// 并自动为uniqueId非_id的集合建唯一索引(account.Name/global的Key(kv)等)
	mongoDb := gentity.NewMongoDb(this.GetConfig().Mongo.Uri, this.GetConfig().Mongo.Db)
	// 账号数据库(不分片)
	// _id=账号名(uniqueId=Name):账号名唯一性由不分片集合的_id主键在数据库层全局原子保证。
	// 业务账号ID改存Id字段(仍由KV自增分配),登录/封禁/在线状态等业务继续以Id为准。
	this.accountDb = db.RegisterAccountDb(mongoDb)
	// 封禁记录数据库
	db.RegisterBanDb(mongoDb)
	// 玩家数据库,用于登录时查询账号在各区服的角色信息
	db.RegisterPlayerDb(mongoDb)
	// global同时注册为EntityDb和KvDb,以便initRegions可以直接用Collection操作;
	// KvDb形态的Key唯一索引由Connect自动创建(闭合KvDb.Inc的upsert并发竞态)
	db.RegisterGlobalEntityDb(mongoDb)
	db.RegisterGlobalKvDb(mongoDb)
	if !mongoDb.Connect() {
		panic(fmt.Sprintf("connect db error,uri:%v db:%v", this.GetConfig().Mongo.Uri, this.GetConfig().Mongo.Db))
	}
	db.SetDbMgr(mongoDb)
	// 分片策略:只对player集合分片(ShardKeyHashed),其余集合ShardKeyNone被框架自动跳过
	// (策略说明与理由见db/db_mgr.go文件头);单机/副本集环境enableSharding命令不存在,
	// 返回错误属预期降级(所有集合不分片,功能不受影响),仅记日志不阻断
	if err := mongoDb.ShardDatabase(this.GetConfig().Mongo.Db); err != nil {
		slog.Info("ShardDatabase skip", "error", err)
	}
	// 账号业务ID普通索引(非唯一:Id唯一性由KV自增分配器保证),
	// 支撑按账号ID查询账号信息的功能
	this.accountDb.(*gentity.MongoCollection).CreateIndex("Id", false)
	// player复合索引{AccountId,RegionId}:登录时queryAccountRegionRoles按AccountId
	// 查角色列表(每次登录必经),无此索引时在每shard上退化为全集合扫描;
	// GameServer启动时也会建(createIndex幂等),此处保证login独立部署时索引不缺
	if err := db.EnsurePlayerAccountRegionIndex(); err != nil {
		panic(fmt.Sprintf("EnsurePlayerAccountRegionIndex err:%v", err))
	}
}

// 初始化redis缓存
func (this *LoginServer) initCache() {
	cache.NewRedis(this.GetConfig().Redis.Uri, this.GetConfig().Redis.UserName, this.GetConfig().Redis.Password, this.GetConfig().Redis.Cluster, this.GetConfig().Redis.DB)
	pong, err := cache.GetRedis().Ping(context.Background()).Result()
	if err != nil || pong == "" {
		panic(fmt.Sprintf("redis connect error,uri:%v err:%v pong:%v", this.GetConfig().Redis.Uri, err, pong))
	}
}

func (this *LoginServer) initNetwork() {
	// NOTE: 实际项目中,监听客户端和监听网关,二选一即可
	// 这里为了演示,同时提供客户端直连和网关两种模式
	if network.ListenClient(this.GetConfig().Client.Addr, nil, this.registerClientPacket) == nil {
		panic("listen client failed")
	}
	if network.ListenGate(this.GetConfig().Gate.Addr, this.registerServerPacket) == nil {
		panic("listen gateserver failed")
	}
	this.GetServerList().SetCache(cache.Get())
	this.BaseServer.GetServerList().SetFetchServerTypes(ServerType_Game)
}

// 启动时初始化区服数据:如果MongoDB中无任何区服则创建默认区服,然后加载进程内存快照
// 区服数据以BSON明文格式存储在global集合中,key="Regions";
// LoginServer只承担"首次建区"的写路径,快照加载/订阅刷新统一走internal.InitRegionCache
func (this *LoginServer) initRegions() {
	// MongoDB为区服数据唯一源,读取统一收敛到db.LoadAllRegions
	// (文档不存在或Value缺失时返回空map,nil error,与首次启动语义一致)
	regions, err := db.LoadAllRegions()
	if err != nil {
		panic(fmt.Sprintf("initRegions load err:%v", err))
	}
	slog.Info("initRegions loaded", "count", len(regions))
	if len(regions) == 0 {
		// 首次启动,创建默认区服
		now := time.Now().Unix()
		defaultRegion := &pb.Region{
			Id:              1,
			Name:            "默认区服",
			Status:          pb.RegionStatus_RegionStatus_Normal,
			CreateTimestamp: now,
			UpdateTimestamp: now,
		}
		regions[defaultRegion.Id] = defaultRegion
		// 以BSON明文map格式保存到mongodb,key=区服id
		regionDocs := make(bson.M, len(regions))
		for id, r := range regions {
			regionDocs[fmt.Sprintf("%v", id)] = bson.M{
				"Id":              r.Id,
				"Name":            r.Name,
				"Status":          int32(r.Status),
				"CreateTimestamp": r.CreateTimestamp,
				"UpdateTimestamp": r.UpdateTimestamp,
			}
		}
		doc := bson.M{
			db.GlobalDbKeyName:   db.RegionsKeyName,
			db.GlobalDbValueName: regionDocs,
		}
		// 首次建区的写路径仍直接操作global集合整份插入(读取已收敛到db.LoadAllRegions)
		mongoCol := db.GetGlobalDb().(*gentity.MongoCollection)
		_, err = mongoCol.GetCollection().InsertOne(context.Background(), doc)
		if err != nil {
			panic(fmt.Sprintf("initRegions insert default region err:%v", err))
		}
		slog.Info("initRegions created default region", "id", defaultRegion.Id, "name", defaultRegion.Name)
		// 建区后发布变更通知:正常部署顺序下其他进程尚未启动(无订阅者,PUBLISH不是错误),
		// 但若GameServer先于LoginServer启动,其快照此时为空map,靠这条通知立即补齐,
		// 不必等到下一次GM区服变更或进程重启
		if err := cache.PublishRegionUpdate(context.Background(), defaultRegion.Id); err != nil {
			// 发布失败仅记日志:本进程随后的InitRegionCache会读到刚插入的数据,
			// 受影响的只是可能已在运行的其他进程,它们重启后自然纠正
			slog.Error("initRegions publish default region failed",
				"regionId", defaultRegion.Id, "error", err)
		}
	}
	// 加载进程内存快照并订阅后续变更通知(与GameServer共用internal的同一套快照)
	if err := InitRegionCache(this.GetContext()); err != nil {
		panic(fmt.Sprintf("initRegions InitRegionCache err:%v", err))
	}
}

// AccountSaveData 构建账号文档的存储数据(自建注册与SDK开户共用,保证字段口径一致)
// _id=账号名:账号名唯一性由分片键(_id)路由+本地唯一索引全局保证(分片集群安全),
// 并发同名注册/开户时,后插入者由MongoDB原子返回E11000;
// Id为KV自增分配的业务账号ID(proto字段,原名_id/XId,分片改造后更名),
// 登录/封禁/在线状态等业务均以Id为准
func AccountSaveData(account *pb.Account) map[string]any {
	return map[string]any{
		db.UniqueIdName: account.Name,     // mongodb _id = 账号名
		db.AccountName:  account.Name,     // 分片键字段(必须存在于每个文档,与_id同值)
		"Id":            account.Id,       // 业务账号ID
		"Password":      account.Password, // 客户端加密后的值,非明文
	}
}

func (this *LoginServer) getAccountData(accountName string, accountData *pb.Account) error {
	mongoCol := this.GetAccountDb().(*gentity.MongoCollection)
	col := mongoCol.GetCollection()
	// 按账号名(_id)查询:主键直达,且与写入的唯一性口径一致
	// (_id即账号名,E11000与查询命中的是同一个键,不存在"索引与主键不一致"的缝隙)
	result := col.FindOne(context.Background(), bson.D{{db.UniqueIdName, accountName}})
	if result == nil || errors.Is(result.Err(), mongo.ErrNoDocuments) {
		return nil
	}
	err := result.Decode(accountData)
	if err != nil {
		return err
	}
	// XId/Name/Password均为文档显式字段,driver解码时按字段名大小写不敏感匹配
	// (无bson tag的struct先按原名后按小写匹配,见mongo-driver struct_codec),
	// 可直接decode到pb.Account对应字段,无需手工从Raw补值
	return nil
}

// 注册客户端消息回调
func (this *LoginServer) registerClientPacket(clientHandler *DefaultConnectionHandler) {
	// 状态检查包装器:非Running状态时拒绝客户端请求
	// 服务器正在退出时返回ServerClosing,其他非运行状态(如初始化中)返回TryLater
	checkRunning := func(handler PacketHandler) PacketHandler {
		return func(connection Connection, packet Packet) {
			if this.IsRunning() {
				handler(connection, packet)
				return
			}
			status := this.GetStatus()
			var errCode pb.ErrorCode
			if status == ServerStatus_Exit {
				errCode = pb.ErrorCode_ErrorCode_ServerClosing
			} else {
				errCode = pb.ErrorCode_ErrorCode_TryLater
			}
			network.SendPacketAdaptWithError(connection, packet, &pb.ErrorRes{
				Command:  int32(packet.Command()),
				ResultId: int32(errCode),
			}, int32(errCode))
		}
	}
	network.RegisterPacketHandler(clientHandler, new(pb.LoginReq), checkRunning(onLoginReq))
	network.RegisterPacketHandler(clientHandler, new(pb.AccountReg), checkRunning(onAccountReg))
}

// 注册服务器消息回调
func (this *LoginServer) registerServerPacket(serverHandler *DefaultConnectionHandler) {
	this.registerClientPacket(serverHandler)
}
