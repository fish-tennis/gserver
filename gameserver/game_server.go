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
	"github.com/fish-tennis/gserver/social"
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
	this.AddServerHook(&game.Hook{}, &social.Hook{})

	this.initDb()
	this.initCache()
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
}

// 初始化数据库
func (this *GameServer) initDb() {
	// 使用mongodb来演示
	mongoDb := gentity.NewMongoDb(this.GetConfig().Mongo.Uri, this.GetConfig().Mongo.Db)
	// 玩家数据库
	mongoDb.RegisterPlayerDb(db.PlayerDbName, true, db.UniqueIdName, db.PlayerAccountId, db.PlayerRegionId)
	// 公会数据库
	mongoDb.RegisterEntityDb(db.GuildDbName, true, db.UniqueIdName)
	// 全局对象数据库(如GlobalEntity)
	mongoDb.RegisterEntityDb(db.GlobalDbName, true, db.GlobalDbKeyName)
	// kv数据库
	mongoDb.RegisterKvDb(db.GlobalDbName, true, db.GlobalDbKeyName, db.GlobalDbValueName)
	if !mongoDb.Connect() {
		panic("connect db error")
	}
	// 玩家数据库设置分片
	mongoDb.ShardDatabase(this.GetConfig().Mongo.Db)
	db.SetDbMgr(mongoDb)
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
	this.GetServerList().SetFetchAndConnectServerTypes(ServerType_Game)
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
		// AddOnlinePlayer 失败说明 keyOnlinePlayer 已存在(可能是上次崩溃的残留数据)
		// AddOnlineAccount 已保证账号独占性,这里清理残留后重新设置,保证内存与Redis状态一致
		_, oldGameServerId := cache.GetOnlinePlayer(player.GetId())
		slog.Error("AddOnlinePlayer stale record, cleaning", "playerId", player.GetId(), "accountId", player.GetAccountId(), "oldGameServerId", oldGameServerId)
		cache.RemoveOnlinePlayer(player.GetId(), oldGameServerId)
		cache.AddOnlinePlayer(player.GetId(), player.GetAccountId(), this.GetId())
	}
}

// 删除一个在线玩家
func (this *GameServer) RemovePlayer(player IPlayer) {
	// 先保存数据库 再移除cache
	player.(*game.Player).SaveDb(true)
	this.playerMap.Delete(player.GetId())
	cache.RemoveOnlineAccount(player.GetAccountId())
	cache.RemoveOnlinePlayer(player.GetId(), this.GetId())
	this.playerWg.Done()
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
			cache.RemoveOnlineAccount(req.AccountId)
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

// 转发玩家消息
// otherServer -> thisServer -> player
func (this *GameServer) onRoutePlayerMessage(connection Connection, packet Packet) {
	req := packet.Message().(*pb.RoutePlayerMessage)
	slog.Debug("onRoutePlayerMessage", "packet", packet)
	player := game.GetPlayer(req.ToPlayerId)
	if player == nil {
		// NOTE: 由于是异步消息,这里的player有很低的概率可能不在线了,如果是重要的不能丢弃的消息,需要保存该消息,留待后续处理
		slog.Error("player nil", "playerId", req.ToPlayerId, "cmd", req.PacketCommand)
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
	if req.DirectSendClient {
		// 不需要player处理的消息,投递到玩家协程内转发给客户端,避免跨协程读 p.connection
		player.DirectSendClient(PacketCommand(uint16(req.PacketCommand)), message)
	} else {
		// 需要player处理的消息,放进player的消息队列,在玩家的逻辑协程中处理
		player.OnRecvPacket(NewProtoPacket(PacketCommand(req.PacketCommand), message))
	}
	if req.PendingMessageId > 0 {
		// 消息保存到db了,处理完需要删除
		game.DeletePendingMessage(req.ToPlayerId, req.PendingMessageId)
	}
}
