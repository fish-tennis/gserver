package gameserver

import (
	"context"
	"encoding/json"
	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/game"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/social"
	"google.golang.org/protobuf/proto"
	"os"
	"sync"
)

var (
	_ gentity.Application = (*GameServer)(nil)
)

// 游戏服
type GameServer struct {
	*BaseServer
	config *GameServerConfig
	// 网关服务器listener
	gateListener Listener
	// 在线玩家
	playerMap sync.Map // playerId-*player
}

// 游戏服配置
type GameServerConfig struct {
	BaseServerConfig
}

func NewGameServer(ctx context.Context, configFile string) *GameServer {
	s := &GameServer{
		BaseServer: NewBaseServer(ctx, ServerType_Game, configFile),
		config:     new(GameServerConfig),
	}
	s.readConfig()
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

	for _, hook := range this.BaseServer.GetServerHooks() {
		hook.OnApplicationInit(nil)
	}
	logger.Info("GameServer.Init")
	return true
}

// 运行
func (this *GameServer) Run(ctx context.Context) {
	this.BaseServer.Run(ctx)
	logger.Info("GameServer.Run")
}

// 退出
func (this *GameServer) Exit() {
	this.playerMap.Range(func(key, value interface{}) bool {
		player := value.(*game.Player)
		player.Stop()
		return true
	})
	game.GetGlobalEntity().PushMessage(NewProtoPacket(PacketCommand(pb.CmdServer_Cmd_ShutdownReq), &pb.ShutdownReq{
		Timestamp: game.GetGlobalEntity().GetTimerEntries().Now().Unix(),
	}))
	this.BaseServer.Exit()
	logger.Info("GameServer.Exit")
	dbMgr := db.GetDbMgr()
	if dbMgr != nil {
		dbMgr.(*gentity.MongoDb).Disconnect()
	}
}

// 读取配置文件
func (this *GameServer) readConfig() {
	fileData, err := os.ReadFile(this.GetConfigFile())
	if err != nil {
		panic("read config file err")
	}
	this.config = new(GameServerConfig)
	err = json.Unmarshal(fileData, this.config)
	if err != nil {
		panic("decode config file err")
	}
	logger.Debug("%v", this.config)
	this.BaseServer.GetServerInfo().ServerId = this.config.ServerId
	this.BaseServer.GetServerInfo().ClientListenAddr = this.config.ClientListenAddr
	this.BaseServer.GetServerInfo().GateListenAddr = this.config.GateListenAddr
	this.BaseServer.GetServerInfo().ServerListenAddr = this.config.ServerListenAddr
}

// 加载配置数据
func (this *GameServer) loadCfgs() {
	progressMgr := game.RegisterProgressCheckers()
	conditionMgr := game.RegisterConditionCheckers()
	cfg.LoadAllCfgs("cfgdata")
	cfg.GetQuestCfgMgr().SetProgressMgr(progressMgr)
	cfg.GetQuestCfgMgr().SetConditionMgr(conditionMgr)
	cfg.GetActivityCfgMgr().SetProgressMgr(progressMgr)
	cfg.GetActivityCfgMgr().SetConditionMgr(conditionMgr)
}

// 初始化数据库
func (this *GameServer) initDb() {
	// 使用mongodb来演示
	mongoDb := gentity.NewMongoDb(this.config.MongoUri, this.config.MongoDbName)
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
	mongoDb.ShardDatabase(this.config.MongoDbName)
	db.SetDbMgr(mongoDb)
}

// 初始化redis缓存
func (this *GameServer) initCache() {
	cache.NewRedis(this.config.RedisUri, this.config.RedisUsername, this.config.RedisPassword, this.config.RedisCluster)
	pong, err := cache.GetRedis().Ping(context.Background()).Result()
	if err != nil || pong == "" {
		panic("redis connect error")
	}
	this.repairCache()
}

func (this *GameServer) initNetwork() {
	// NOTE: 实际项目中,监听客户端和监听网关,二选一即可
	// 这里为了演示,同时提供客户端直连和网关两种模式
	if network.ListenClient(this.config.ClientListenAddr, &ClientListerHandler{}, this.registerClientPacket) == nil {
		panic("listen client failed")
	}

	this.gateListener = network.ListenGate(this.config.GateListenAddr, this.registerGatePacket)
	if this.gateListener == nil {
		panic("listen gate failed")
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
	if this.GetServerList().StartListen(this.GetContext(), this.config.ServerListenAddr) == nil {
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
			logger.Error("repairPlayerCache %v err:%v", playerId, err.(error).Error())
			LogStack()
		}
	}()
	tmpPlayer := game.CreateTempPlayer(playerId, accountId)
	gentity.FixEntityDataFromCache(tmpPlayer, db.GetPlayerDb(), cache.Get(), game.PlayerCachePrefix, playerId)
	return nil
}

// 注册客户端消息回调
func (this *GameServer) registerClientPacket(clientHandler *DefaultConnectionHandler) {
	// 手动注册特殊的消息回调
	clientHandler.Register(PacketCommand(pb.CmdClient_Cmd_PlayerEntryGameReq), onPlayerEntryGameReq, new(pb.PlayerEntryGameReq))
	clientHandler.Register(PacketCommand(pb.CmdClient_Cmd_CreatePlayerReq), onCreatePlayerReq, new(pb.CreatePlayerReq))
	this.registerGatePlayerPacket(clientHandler, PacketCommand(pb.CmdInner_Cmd_TestCmd), onTestCmd, new(pb.TestCmd))
	// 通过反射自动注册消息回调
	game.AutoRegisterPlayerPacketHandler(clientHandler)
}

// 注册网关消息回调
func (this *GameServer) registerGatePacket(gateHandler *DefaultConnectionHandler) {
	// 手动注册特殊的消息回调
	gateHandler.Register(PacketCommand(pb.CmdClient_Cmd_PlayerEntryGameReq), onPlayerEntryGameReq, new(pb.PlayerEntryGameReq))
	gateHandler.Register(PacketCommand(pb.CmdClient_Cmd_CreatePlayerReq), onCreatePlayerReq, new(pb.CreatePlayerReq))
	gateHandler.Register(PacketCommand(pb.CmdInner_Cmd_ClientDisconnect), onClientDisconnect, new(pb.TestCmd))
	this.registerGatePlayerPacket(gateHandler, PacketCommand(pb.CmdInner_Cmd_TestCmd), onTestCmd, new(pb.TestCmd))
	gateHandler.SetUnRegisterHandler(func(connection Connection, packet Packet) {
		if gatePacket, ok := packet.(*network.GatePacket); ok {
			playerId := gatePacket.PlayerId()
			player := this.GetPlayer(playerId)
			if player != nil {
				if gamePlayer, ok := player.(*game.Player); ok {
					if gamePlayer.GetConnection() == connection {
						// 在线玩家的消息,转到玩家消息处理协程去处理
						gamePlayer.OnRecvPacket(gatePacket.ToProtoPacket())
						logger.Debug("gate->player playerId:%v cmd:%v", playerId, packet.Command())
					}
				}
			}
		}
	})
	// 网关服务器掉线,该网关上的所有玩家都掉线
	gateHandler.SetOnDisconnectedFunc(func(connection Connection) {
		this.playerMap.Range(func(key, value interface{}) bool {
			if player, ok := value.(*game.Player); ok {
				if player.GetConnection() == connection {
					player.OnDisconnect(connection)
				}
			}
			return true
		})
	})
	// 通过反射自动注册消息和proto.Message的映射
	game.AutoRegisterPlayerPacketHandler(gateHandler)
}

// 注册func(player *Player, packet Packet)格式的消息回调函数,支持网关模式和客户端直连模式
func (this *GameServer) registerGatePlayerPacket(gateHandler PacketHandlerRegister, packetCommand PacketCommand, playerHandler func(player *game.Player, packet Packet), protoMessage proto.Message) {
	gateHandler.Register(packetCommand, func(connection Connection, packet Packet) {
		var playerId int64
		if gatePacket, ok := packet.(*network.GatePacket); ok {
			// 网关转发的消息,包含playerId
			playerId = gatePacket.PlayerId()
		} else {
			// 客户端直连的模式
			if connection.GetTag() == nil {
				return
			}
			playerId, ok = connection.GetTag().(int64)
			if !ok {
				return
			}
		}
		player := game.GetPlayer(playerId)
		if player == nil {
			return
		}
		// 网关模式,使用的GatePacket
		if gatePacket, ok := packet.(*network.GatePacket); ok {
			// 转换成ProtoPacket,业务层统一接口
			player.OnRecvPacket(gatePacket.ToProtoPacket())
		} else {
			// 客户端直连模式,使用的ProtoPacket
			player.OnRecvPacket(packet.(*ProtoPacket))
		}
	}, protoMessage)
	game.RegisterPlayerHandler(packetCommand, playerHandler)
}

// 注册服务器消息回调
func (this *GameServer) registerServerPacket(handler *DefaultConnectionHandler) {
	handler.Register(PacketCommand(pb.CmdInner_Cmd_KickPlayer), this.onKickPlayer, new(pb.KickPlayer))
	handler.Register(PacketCommand(pb.CmdServer_Cmd_RoutePlayerMessage), this.onRoutePlayerMessage, new(pb.RoutePlayerMessage))
	//serverHandler.autoRegisterPlayerComponentProto()
}

// 添加一个在线玩家
func (this *GameServer) AddPlayer(player IPlayer) {
	this.playerMap.Store(player.GetId(), player)
	cache.AddOnlinePlayer(player.GetId(), player.GetAccountId(), this.GetId())
}

// 删除一个在线玩家
func (this *GameServer) RemovePlayer(player IPlayer) {
	// 先保存数据库 再移除cache
	player.(*game.Player).SaveDb(true)
	this.playerMap.Delete(player.GetId())
	cache.RemoveOnlineAccount(player.GetAccountId())
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
func (this *GameServer) onKickPlayer(connection Connection, packet Packet) {
	req := packet.Message().(*pb.KickPlayer)
	player := game.GetPlayer(req.GetPlayerId())
	if player != nil {
		player.ResetConnection()
		player.Stop()
		logger.Debug("kick player account:%v playerId:%v gameServerId:%v",
			req.GetAccountId(), req.GetPlayerId(), this.GetId())
	} else {
		playerId, gameServerId := cache.GetOnlineAccount(req.AccountId)
		if playerId == req.PlayerId && gameServerId == this.GetId() {
			cache.RemoveOnlineAccount(req.AccountId)
			logger.Info("kick player2 account:%v playerId:%v gameServerId:%v",
				req.GetAccountId(), req.GetPlayerId(), this.GetId())
		} else {
			logger.Error("kick player failed account:%v playerId:%v gameServerId:%v",
				req.GetAccountId(), req.GetPlayerId(), this.GetId())
		}
	}
	// rpc reply
	connection.SendPacket(NewProtoPacket(packet.Command(), req).WithRpc(packet))
}

// 转发玩家消息
// otherServer -> thisServer -> player
func (this *GameServer) onRoutePlayerMessage(connection Connection, packet Packet) {
	req := packet.Message().(*pb.RoutePlayerMessage)
	logger.Debug("onRoutePlayerMessage %v", packet)
	player := game.GetPlayer(req.ToPlayerId)
	if player == nil {
		// NOTE: 由于是异步消息,这里的player有很低的概率可能不在线了,如果是重要的不能丢弃的消息,需要保存该消息,留待后续处理
		// 演示程序暂不处理,这里就直接丢弃了
		logger.Error("player nil %v,cmd:%v", req.ToPlayerId, req.PacketCommand)
		return
	}
	if req.PacketData == nil {
		logger.Error("onRoutePlayerMessage playerId:%v,cmd:%v errStr:%v", req.ToPlayerId, req.PacketCommand, req.Error)
		return
	}
	message, err := req.PacketData.UnmarshalNew()
	if err != nil {
		logger.Error("UnmarshalNew %v cmd:%v err:%v", req.ToPlayerId, req.PacketCommand, err)
		return
	}
	err = req.PacketData.UnmarshalTo(message)
	if err != nil {
		logger.Error("UnmarshalTo %v cmd:%v err:%v DirectSendClient:%v", req.ToPlayerId, req.PacketCommand, err, req.DirectSendClient)
		return
	}
	if req.DirectSendClient {
		// 不需要player处理的消息,直接转发给客户端
		player.SendWithCommand(PacketCommand(uint16(req.PacketCommand)), message)
	} else {
		// 需要player处理的消息,放进player的消息队列,在玩家的逻辑协程中处理
		player.OnRecvPacket(NewProtoPacket(PacketCommand(req.PacketCommand), message))
	}
	if req.PendingMessageId > 0 {
		// 消息保存到db了,处理完需要删除
		game.DeletePendingMessage(req.ToPlayerId, req.PendingMessageId)
	}
}
