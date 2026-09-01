package gameserver

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/game"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
)

var (
	_ gentity.Application = (*GameServer)(nil)
)

// 游戏服
type GameServer struct {
	*BaseServer
	// 网关服务器listener
	gateListener Listener
	// 在线玩家
	playerMap sync.Map // playerId-*player
	// 用于Exit时等待所有玩家协程完成EndFunc(SaveDb+RemovePlayer)后再关闭基础设施
	// AddPlayer时Add(1),RemovePlayer时Done()
	playerWg sync.WaitGroup
}

func NewGameServer(ctx context.Context, configFile string, cfgDir string) *GameServer {
	s := &GameServer{
		BaseServer: NewBaseServer(ctx, ServerType_Game, configFile, cfgDir),
	}
	s.ReadConfig()
	return s
}

// 初始化
func (this *GameServer) Init(ctx context.Context, configFile string) bool {
	game.SetPlayerMgr(this)
	if !this.BaseServer.Init(ctx, configFile) {
		return false
	}
	this.loadCfgs()

	game.InitPlayerStructAndHandler()
	// 其他模块初始化接口
	this.AddServerHook(&game.Hook{})

	this.initDb()
	this.initCache()
	// 订阅热更配置通知,收到通知后按md5快照diff选择性重载本进程配置表
	cache.SubscribeReloadConfig(this.GetContext(), func() {
		if err := cfg.Reload(this.GetCfgDir()); err != nil {
			slog.Error("GameServer reload config failed", "error", err)
		} else {
			slog.Info("GameServer config reloaded")
		}
	})
	// 预加载区服数据到内存缓存并订阅变更通知(订阅逻辑已收敛在InitRegionCache内部)
	if err := InitRegionCache(this.GetContext()); err != nil {
		slog.Error("InitRegionCache failed", "error", err)
	}
	this.initNetwork()
	// 初始化DB操作协程池,将进游/创角等含DB查询的请求从收包goroutine卸载
	InitDbWorkerPool()

	for _, hook := range this.BaseServer.GetServerHooks() {
		hook.OnApplicationInit(nil)
	}
	slog.Info("GameServer.Init")
	return true
}

// 运行
func (this *GameServer) Run(ctx context.Context) {
	this.BaseServer.Run(ctx)
	slog.Info("GameServer.Run")
}

// 退出
func (this *GameServer) Exit() {
	this.SetStatus(ServerStatus_Exit)
	// 先关闭DB协程池并等待在途任务(含AddPlayer)全部完成:
	// 新请求已被状态检查拒绝,不会再投递新任务到协程池
	// 若不先关闭,在途的AddPlayer可能在下面的Range之后才执行,
	// 导致该玩家playerWg.Add(1)了却没被Stop,playerWg.Wait()永久阻塞
	ShutdownDbWorkerPool()
	this.playerMap.Range(func(key, value interface{}) bool {
		player := value.(*game.Player)
		player.Stop()
		return true
	})
	cmd := network.GetCommandByProto(new(pb.ShutdownReq))
	game.GetGlobalEntity().PushMessage(NewProtoPacket(PacketCommand(cmd), &pb.ShutdownReq{
		Timestamp: game.GetGlobalEntity().GetTimerEntries().Now().Unix(),
	}))
	// 等待所有玩家协程完成 EndFunc(SaveDb + RemovePlayer)
	// player.Stop() 是异步的(只发停止信号),若不等待就直接关闭 Redis/Mongo,
	// EndFunc 中的 SaveDb 和 cache 清理会失败,导致数据丢失或 Redis 残留
	this.playerWg.Wait()
	this.BaseServer.Exit()
	slog.Info("GameServer.Exit")
	dbMgr := db.GetDbMgr()
	if dbMgr != nil {
		dbMgr.(*gentity.MongoDb).Disconnect()
	}
}

// 加载配置数据
func (this *GameServer) loadCfgs() {
	err := cfg.Load(this.GetCfgDir(), nil)
	if err != nil {
		panic(fmt.Sprintf("loadCfgs:%v", err))
	}
	// 启动加载成功后建立md5快照,首次热更即走增量路径(见cfg/reload.go)
	cfg.InitMd5Snapshot(this.GetCfgDir())
}

// 初始化数据库
func (s *GameServer) initDb() {
	// 使用mongodb来演示
	// 顺序要求:必须先Register再Connect——框架Connect会回填各集合的client/db句柄,
	// 并自动为uniqueId非_id的集合建唯一索引(global_mail.MailId/global的Key(kv)等)
	mongoDb := gentity.NewMongoDb(s.GetConfig().Mongo.Uri, s.GetConfig().Mongo.Db)
	// 玩家数据库(全项目唯一分片集合,ShardKeyHashed)
	playerDb := db.RegisterPlayerDb(mongoDb)
	// 公会数据库
	db.RegisterGuildDb(mongoDb)
	// 全局对象数据库(如GlobalEntity),global同时注册EntityDb和KvDb,
	// KvDb形态的Key唯一索引由Connect自动创建(闭合KvDb.Inc的upsert并发竞态)
	db.RegisterGlobalEntityDb(mongoDb)
	db.RegisterGlobalKvDb(mongoDb)
	// 封禁记录数据库
	db.RegisterBanDb(mongoDb)
	// 账号区服注册表:建角防重的原子抢占集合(设计说明见db/player_account.go)。
	// 只有GameServer处理建角,其余进程(api/pay/login)不访问该集合,无需注册
	db.RegisterPlayerAccountDb(mongoDb)
	if !mongoDb.Connect() {
		panic("connect db error")
	}
	// 先设置全局DbMgr:后续EnsureSdkOrderIdIndex等函数内部经GetDbMgr()取单例,
	// 若在SetDbMgr之前调用会是nil导致panic
	db.SetDbMgr(mongoDb)
	// 分片策略:只对player集合分片(ShardKeyHashed),其余集合ShardKeyNone被框架自动跳过
	// (策略说明与理由见db/db_mgr.go文件头);单机/副本集环境enableSharding命令不存在,
	// 返回错误属预期降级(所有集合不分片,功能不受影响),仅记日志不阻断
	if err := mongoDb.ShardDatabase(s.GetConfig().Mongo.Db); err != nil {
		slog.Info("ShardDatabase skip", "error", err)
	}
	// 为 regionid 创建索引
	if colPlayer, ok := playerDb.(*gentity.MongoCollectionPlayer); ok {
		colPlayer.CreateIndex(db.PlayerRegionId, false)
	}
	// player复合索引{AccountId,RegionId}:登录/进服/建角的账号查询走索引直达,
	// 否则分片集群下每次登录都在所有shard上全表扫描(见db/db_mgr.go的设计说明)
	if err := db.EnsurePlayerAccountRegionIndex(); err != nil {
		panic(fmt.Sprintf("EnsurePlayerAccountRegionIndex err:%v", err))
	}
	// 注册表TTL索引:自动清理建角中断(crash/panic)留下的注册位残留,
	// 消除"抢占成功后进程崩溃导致该账号永远无法建角"的泄漏问题;
	// 分片集合的TTL索引在各shard本地清理,createIndex自动传播,无需特殊处理。
	// NOTE:必须在SetDbMgr之后调用(内部经GetDbMgr取单例,见上方注释)
	if err := db.EnsurePlayerAccountTtlIndex(); err != nil {
		panic(fmt.Sprintf("EnsurePlayerAccountTtlIndex err:%v", err))
	}
}

// 初始化redis缓存
func (this *GameServer) initCache() {
	cache.NewRedis(this.GetConfig().Redis.Uri, this.GetConfig().Redis.UserName, this.GetConfig().Redis.Password, this.GetConfig().Redis.Cluster, this.GetConfig().Redis.DB)
	pong, err := cache.GetRedis().Ping(context.Background()).Result()
	if err != nil || pong == "" {
		panic("redis connect error")
	}
	this.repairCache()
}

func (this *GameServer) initNetwork() {
	// NOTE: 实际项目中,监听客户端和监听网关,二选一即可
	// 这里为了演示,同时提供客户端直连和网关两种模式
	if network.ListenClient(this.GetConfig().Client.Addr, &ClientListerHandler{}, this.registerClientPacket) == nil {
		panic("listen client failed")
	}

	// 网关比较特殊,单独处理
	this.gateListener = network.ListenGate(this.GetConfig().Gate.Addr, this.registerGatePacket)
	if this.gateListener == nil {
		panic("listen gateserver failed")
	}

	this.GetServerList().SetCache(cache.Get())
	// 注册业务层的消息回调
	serverHandlers := []*DefaultConnectionHandler{
		this.GetServerList().GetServerConnectionHandler(),
		this.GetServerList().GetServerListenerHandler(),
	}
	for _, serverHandler := range serverHandlers {
		this.registerServerPacket(serverHandler)
		// 其他模块注册服务器之间的消息回调
		for _, hook := range this.GetServerHooks() {
			hook.OnRegisterServerHandler(serverHandler)
		}
	}
	this.GetServerList().SetFetchAndConnectServerTypes(ServerType_Game, ServerType_Social)
	// 通用的服务器间的监听
	if this.GetServerList().StartListen(this.GetContext(), this.GetConfig().Server.Addr) == nil {
		panic("listen server failed")
	}
}

// 修复缓存,游戏服异常宕机重启后进行修复操作
func (this *GameServer) repairCache() {
	cache.ResetOnlinePlayer(this.GetId(), this.repairPlayerCache)
}

// 缓存中的玩家数据保存到数据库
// 服务器crash时,缓存数据没来得及保存到数据库,服务器重启后进行自动修复,防止玩家数据回档
func (this *GameServer) repairPlayerCache(playerId, accountId int64) error {
	defer func() {
		if err := recover(); err != nil {
			slog.Error("repairPlayerCache error", "playerId", playerId, "err", err)
			LogStack()
			SendAlert(err)
		}
	}()
	tmpPlayer := game.CreateTempPlayer(playerId, accountId)
	gentity.FixEntityDataFromCache(tmpPlayer, db.GetPlayerDb(), cache.Get(), game.PlayerCachePrefix, playerId)
	return nil
}

// 注册客户端消息回调
func (this *GameServer) registerClientPacket(handler *DefaultConnectionHandler) {
	// 状态检查包装器:非Running状态时拒绝客户端请求,避免在服务器初始化或退出阶段处理进游/创角等请求
	checkRunning := func(handler PacketHandler) PacketHandler {
		return func(connection Connection, packet Packet) {
			if this.IsRunning() {
				handler(connection, packet)
				return
			}
			// 服务器正在关闭时告知客户端ServerClosing,其他非Running状态(如初始化中)告知TryLater
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
	// 手动注册特殊的消息回调,用checkRunning包装以拦截非Running状态的请求
	network.RegisterPacketHandler(handler, new(pb.PlayerEntryGameReq), checkRunning(onPlayerEntryGameReq))
	network.RegisterPacketHandler(handler, new(pb.PlayerReconnectGameReq), checkRunning(onPlayerReconnectGameReq))
	network.RegisterPacketHandler(handler, new(pb.CreatePlayerReq), checkRunning(onCreatePlayerReq))
	handler.SetUnRegisterHandler(func(connection Connection, packet Packet) {
		// 非Running状态时拒绝请求,防止服务器退出阶段继续路由消息到玩家协程
		if !this.IsRunning() {
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
			return
		}
		var playerId int64
		var playerPacket *ProtoPacket
		if gatePacket, ok := packet.(*network.GatePacket); ok {
			// 网关转发的消息,包含playerId
			playerId = gatePacket.PlayerId()
			playerPacket = gatePacket.ToProtoPacket()
		} else {
			// 客户端直连的模式
			if connection.GetTag() == nil {
				return
			}
			playerId, ok = connection.GetTag().(int64)
			if !ok {
				return
			}
			playerPacket = packet.(*ProtoPacket)
		}
		player := this.GetPlayer(playerId)
		if player == nil {
			slog.Debug("playerNil", "playerId", playerId, "packet", packet)
			// 告诉网关,这个玩家不在本服务器上
			network.SendPacketAdapt(connection, packet, &pb.GateRouteClientPacketError{
				PlayerId:  playerId,
				Command:   int32(packet.Command()),
				ResultStr: "PlayerNil",
			})
			return
		}
		if gamePlayer, ok := player.(*game.Player); ok {
			// 投递到玩家协程内验证连接归属后处理,避免跨协程读 p.connection
			// 使用 TryPushMessage 非阻塞投递:channel 满时返回错误给客户端,防止阻塞 gate 收包协程
			if !gamePlayer.CheckConnectionAndRecvClientPacket(connection, playerPacket) {
				slog.Warn("player channel full, dropping client message", "playerId", playerId, "cmd", packet.Command())
				network.SendPacketAdaptWithError(connection, packet, &pb.ErrorRes{
					Command:   int32(packet.Command()),
					ResultStr: "ServerBusy",
				}, int32(pb.ErrorCode_ErrorCode_PushClientPacketLoss))
			}
		}
	})
	// 通过反射自动注册消息回调
	game.AutoRegisterPlayerPacketHandler(handler)
}

// 注册网关消息回调
func (this *GameServer) registerGatePacket(handler *DefaultConnectionHandler) {
	this.registerClientPacket(handler)
	network.RegisterPacketHandler(handler, new(pb.ClientDisconnect), onClientDisconnect)
	network.RegisterPacketHandler(handler, new(pb.ServerHello), onGateHello)
}

// 注册服务器消息回调
func (this *GameServer) registerServerPacket(handler *DefaultConnectionHandler) {
	network.RegisterPacketHandler(handler, new(pb.KickPlayerReq), this.onKickPlayer)
	network.RegisterPacketHandler(handler, new(pb.RoutePlayerMessage), this.onRoutePlayerMessage)
}

// 添加一个在线玩家
func (this *GameServer) AddPlayer(player IPlayer) {
	this.playerWg.Add(1)
	this.playerMap.Store(player.GetId(), player)
	if !cache.AddOnlinePlayer(player.GetId(), player.GetAccountId(), this.GetId()) {
		// 占有失败说明记录被其他服务器持有
		// 本服已通过 AddOnlineAccount 获得账号独占,记录持有者只可能是:
		// 1) 已宕机服务器的崩溃残留 2) 正在下线的旧服务器的中间态记录
		// 旧服务器的下线清理是条件释放(值匹配才删),不会误删这里的新记录,因此强制接管是安全的
		// 原子接管(读取旧值+写入新值)消除了原先 Get->Remove->Add 三步之间的check-then-act竞态
		// AddOnlinePlayer占有失败时不回滚SAdd,接管后记录与本服集合索引保持一致,
		// 本服宕机重启后ResetOnlinePlayer可从集合索引找到该玩家并自修复
		oldAccountId, oldGameServerId := cache.TakeOverOnlinePlayer(player.GetId(), player.GetAccountId(), this.GetId())
		slog.Error("AddOnlinePlayer stale record, takeover",
			"playerId", player.GetId(), "accountId", player.GetAccountId(),
			"oldAccountId", oldAccountId, "oldGameServerId", oldGameServerId)
	}
}

// 删除一个在线玩家
func (this *GameServer) RemovePlayer(player IPlayer) {
	// defer 保证 playerWg.Done 一定执行,即使 SaveDb 或 cache 清理 panic 也不会导致 Exit 的 playerWg.Wait 永久阻塞
	defer this.playerWg.Done()
	// 先保存数据库 再移除cache
	if err := player.(*game.Player).SaveDb(true); err != nil {
		slog.Error("RemovePlayer SaveDb error", "playerId", player.GetId(), "error", err)
	}
	this.playerMap.Delete(player.GetId())
	// 条件释放:仅当记录仍属于本服时才删除,防止误删新服务器已写入的新记录
	cache.RemoveOnlineAccount(player.GetAccountId(), player.GetId(), this.GetId())
	cache.RemoveOnlinePlayer(player.GetId(), this.GetId())
}

// 获取一个在线玩家
func (this *GameServer) GetPlayer(playerId int64) IPlayer {
	if v, ok := this.playerMap.Load(playerId); ok {
		return v.(IPlayer)
	}
	return nil
}

// 踢玩家下线
// TODO: GameServer也建一个实体,把onKickPlayer放到组件rpc回调上
func (this *GameServer) onKickPlayer(connection Connection, packet Packet) {
	req := packet.Message().(*pb.KickPlayerReq)
	player := game.GetPlayer(req.GetPlayerId())
	if player != nil {
		// 投递到玩家协程内执行 ResetConnection + Stop,避免跨协程访问 connection
		player.Kick()
		slog.Debug("kick player account", "accountId", req.GetAccountId(), "playerId", req.GetPlayerId(), "gameServerId", this.GetId())
	} else {
		playerId, gameServerId := cache.GetOnlineAccount(req.AccountId)
		if playerId == req.PlayerId && gameServerId == this.GetId() {
			// 条件释放:防止Get与删除之间记录被其他流程改写后误删
			cache.RemoveOnlineAccount(req.AccountId, req.PlayerId, this.GetId())
			// player==nil时也要清Redis online player key,防止残留导致无法登录
			cache.RemoveOnlinePlayer(req.GetPlayerId(), this.GetId())
			slog.Info("kick player2", "accountId", req.GetAccountId(), "playerId", req.GetPlayerId(), "gameServerId", this.GetId())
		} else {
			slog.Error("kick player failed", "accountId", req.GetAccountId(), "playerId", req.GetPlayerId(), "gameServerId", this.GetId())
		}
	}
	if packet.RpcCallId() == 0 {
		return
	}
	// rpc reply:回复KickPlayerRes,command和类型必须与发起方期望的reply一致
	resCmd := network.GetCommandByProto(new(pb.KickPlayerRes))
	connection.SendPacket(NewProtoPacket(PacketCommand(resCmd), &pb.KickPlayerRes{
		AccountId: req.GetAccountId(),
		PlayerId:  req.GetPlayerId(),
	}).WithRpc(packet))
}

// 转发玩家消息 NOTE:在服务器之间的网络协程中调用,需要避免阻塞服务器间收包协程
// otherServer -> thisServer -> player
func (this *GameServer) onRoutePlayerMessage(connection Connection, packet Packet) {
	req := packet.Message().(*pb.RoutePlayerMessage)
	slog.Debug("onRoutePlayerMessage", "packet", packet)
	player := game.GetPlayer(req.ToPlayerId)
	if player == nil {
		// NOTE: 由于是异步消息,这里的player有很低的概率可能不在线了,如果是重要的不能丢弃的消息,需要保存该消息,留待后续处理
		slog.Error("player nil", "playerId", req.ToPlayerId, "cmd", req.PacketCommand)
		// Rpc调用需要回复错误,避免调用方一直等待
		if packet.RpcCallId() > 0 {
			errReply := network.NewPacket(&pb.RoutePlayerMessage{Error: "player offline"})
			errReply.SetRpcCallId(packet.RpcCallId())
			connection.SendPacket(errReply)
		}
		return
	}
	if req.PacketData == nil {
		slog.Error("onRoutePlayerMessage error", "playerId", req.ToPlayerId, "cmd", req.PacketCommand, "error", req.Error)
		return
	}
	message, err := req.PacketData.UnmarshalNew()
	if err != nil {
		slog.Error("UnmarshalNew error", "playerId", req.ToPlayerId, "cmd", req.PacketCommand, "error", err)
		return
	}
	pushed := true
	if req.Options & int32(pb.RouteOption_RouteOption_DirectSendClient) != 0 {
		// 不需要player处理的消息,投递到玩家协程内转发给客户端,避免跨协程读 p.connection
		// 使用 TryPushMessage 非阻塞投递:channel 满时丢弃并告警,防止阻塞服务器间收包协程
		pushed = player.TryPushMessage(&game.PlayerDirectSendMessage{Cmd: PacketCommand(uint16(req.PacketCommand)), Message: message})
		if !pushed {
			slog.Warn("onRoutePlayerMessage player channel full, dropping DirectSendClient",
				"playerId", req.ToPlayerId, "cmd", req.PacketCommand, "message", message)
		}
	} else {
		// 需要player处理的消息
		if packet.RpcCallId() > 0 {
			// RpcCallId > 0: 需要reply,投递PlayerRouteMessage携带来源connection和rpcCallId
			// 玩家协程执行完handler后通过来源connection做rpc reply
			pushed = player.TryPushMessage(&game.PlayerRouteMessage{
				Cmd:        PacketCommand(req.PacketCommand),
				Message:    message,
				RpcCallId:  packet.RpcCallId(),
				Connection: connection,
			})
			if !pushed {
				slog.Warn("onRoutePlayerMessage player channel full, dropping route rpc",
					"playerId", req.ToPlayerId, "cmd", req.PacketCommand)
			}
		} else {
			// RpcCallId == 0: 无需reply,放进player的消息队列,在玩家的逻辑协程中处理
			// 这是使用 TryPushMessage 非阻塞投递:channel 满时丢弃并告警,防止阻塞服务器间收包协程
			// 如果是重要的不能丢弃的消息,应该设置PendingMessageId,留待玩家下次上线重试
			pushed = player.TryPushMessage(NewProtoPacket(PacketCommand(req.PacketCommand), message))
			if !pushed {
				slog.Warn("onRoutePlayerMessage player channel full, dropping message",
					"playerId", req.ToPlayerId, "cmd", req.PacketCommand, "message", message)
			}
		}
	}
	if pushed && req.PendingMessageId > 0 {
		// 投递成功才删除待发消息;投递失败时保留,留待玩家下次上线重试
		game.DeletePendingMessage(req.ToPlayerId, req.PendingMessageId)
	}
}
