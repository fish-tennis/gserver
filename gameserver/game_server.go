package gameserver

import (
	"context"
	"encoding/json"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/game"
	"github.com/fish-tennis/gserver/social"
	"os"
	"sync"
	"time"

	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/db"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/misc"
	"github.com/fish-tennis/gserver/pb"
)

var (
	_ gentity.Application = (*GameServer)(nil)
)

// 游戏服
type GameServer struct {
	BaseServer
	config *GameServerConfig
	// 网关服务器listener
	gateListener Listener
	// 服务器listener
	serverListener Listener
	// 在线玩家
	playerMap sync.Map // playerId-*player
}

// 游戏服配置
type GameServerConfig struct {
	BaseServerConfig
}

// 初始化
func (this *GameServer) Init(ctx context.Context, configFile string) bool {
	game.SetPlayerMgr(this)
	if !this.BaseServer.Init(ctx, configFile) {
		return false
	}
	this.readConfig()
	this.loadCfgs()
	game.InitPlayerComponentMap()
	this.initDb()
	this.initCache()
	this.GetServerList().SetCache(cache.Get())

	netMgr := GetNetMgr()
	// NOTE: 实际项目中,监听客户端和监听网关,二选一即可
	// 这里为了演示,同时提供客户端直连和网关两种模式
	// 客户端的codec和handler
	clientCodec := NewProtoCodec(nil)
	clientHandler := game.NewClientConnectionHandler(clientCodec)
	this.registerClientPacket(clientHandler)

	if netMgr.NewListener(ctx, this.config.ClientListenAddr, this.config.ClientConnConfig, clientCodec,
		clientHandler, &ClientListerHandler{}) == nil {
		panic("listen client failed")
		return false
	}

	// NOTE: 实际项目中,监听客户端和监听网关,二选一即可
	// 这里为了演示,同时提供客户端直连和网关两种模式
	// 服务器的codec和handler
	gateCodec := NewGateCodec(nil)
	gateHandler := NewDefaultConnectionHandler(gateCodec)
	this.registerGatePacket(gateHandler)
	this.gateListener = netMgr.NewListener(ctx, this.config.GateListenAddr, this.config.ServerConnConfig, gateCodec,
		gateHandler, nil)
	if this.gateListener == nil {
		panic("listen gate failed")
		return false
	}

	// 服务器的codec和handler
	serverCodec := NewProtoCodec(nil)
	serverHandler := NewDefaultConnectionHandler(serverCodec)
	this.registerServerPacket(serverHandler)
	this.serverListener = netMgr.NewListener(ctx, this.config.ServerListenAddr, this.config.ServerConnConfig, serverCodec,
		serverHandler, nil)
	if this.serverListener == nil {
		panic("listen server failed")
		return false
	}

	// 连接其他服务器
	this.BaseServer.SetDefaultServerConnectorConfig(this.config.ServerConnConfig, NewProtoCodec(nil))
	this.BaseServer.GetServerList().SetFetchAndConnectServerTypes(ServerType_Game)

	// 其他模块初始化
	this.AddServerHook(&social.Hook{})
	serverInitArg := &misc.GameServerInitArg{
		ClientHandler: clientHandler,
		ServerHandler: serverHandler,
		PlayerMgr:     game.GetPlayerMgr(),
	}
	for _, hook := range this.BaseServer.GetServerHooks() {
		hook.OnApplicationInit(serverInitArg)
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
	this.BaseServer.GetServerInfo().ServerType = ServerType_Game
	this.BaseServer.GetServerInfo().ClientListenAddr = this.config.ClientListenAddr
	this.BaseServer.GetServerInfo().GateListenAddr = this.config.GateListenAddr
	this.BaseServer.GetServerInfo().ServerListenAddr = this.config.ServerListenAddr
}

// 加载配置数据
func (this *GameServer) loadCfgs() {
	cfg.GetQuestCfgMgr().SetConditionMgr(game.RegisterConditionCheckers())
	cfg.GetQuestCfgMgr().Load("cfgdata/questcfg.json")
	cfg.GetLevelCfgMgr().Load("cfgdata/levelcfg.csv")
	cfg.GetItemCfgMgr().Load("cfgdata/itemcfg.json")
}

// 初始化数据库
func (this *GameServer) initDb() {
	// 使用mongodb来演示
	mongoDb := gentity.NewMongoDb(this.config.MongoUri, this.config.MongoDbName)
	playerDB := mongoDb.RegisterPlayerDb("player", "_id", "accountid", "regionid")
	mongoDb.RegisterEntityDb("guild", "_id")
	if !mongoDb.Connect() {
		panic("connect db error")
	}
	// 玩家数据库设置分片
	mongoDb.ShardDatabase(this.config.MongoDbName)
	playerDB.(*gentity.MongoCollectionPlayer).ShardCollection(true)
	db.SetDbMgr(mongoDb)
}

// 初始化redis缓存
func (this *GameServer) initCache() {
	cache.NewRedis(this.config.RedisUri, this.config.RedisPassword, this.config.RedisCluster)
	pong, err := cache.GetRedis().Ping(context.Background()).Result()
	if err != nil || pong == "" {
		panic("redis connect error")
	}
	this.repairCache()
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
	gentity.FixEntityDataFromCache(tmpPlayer, db.GetPlayerDb(), cache.Get(), "p")
	return nil
}

// 注册客户端消息回调
func (this *GameServer) registerClientPacket(clientHandler PacketHandlerRegister) {
	// 手动注册消息回调
	clientHandler.Register(PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), onHeartBeatReq, new(pb.HeartBeatReq))
	clientHandler.Register(PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameReq), onPlayerEntryGameReq, new(pb.PlayerEntryGameReq))
	clientHandler.Register(PacketCommand(pb.CmdLogin_Cmd_CreatePlayerReq), onCreatePlayerReq, new(pb.CreatePlayerReq))
	clientHandler.Register(PacketCommand(pb.CmdInner_Cmd_TestCmd), wrapPlayerHandler(onTestCmd), new(pb.TestCmd))
	// 通过反射自动注册消息回调
	game.AutoRegisterPlayerComponentProto(clientHandler)
	// 自动注册消息回调的另一种方案: proto_code_gen工具生成的回调函数
	// 因为已经用了反射自动注册,所以这里注释了
	// player_component_handler_gen(clientHandler)
}

func wrapPlayerHandler(fn func(player *game.Player, packet Packet)) func(connection Connection, packet Packet) {
	return func(connection Connection, packet Packet) {
		var playerId int64
		if gatePacket,ok := packet.(*GatePacket); ok {
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
		if gatePacket,ok := packet.(*GatePacket); ok {
			// 转换成ProtoPacket,业务层统一接口
			player.OnRecvPacket(gatePacket.ToProtoPacket())
		} else {
			// 客户端直连模式,使用的ProtoPacket
			player.OnRecvPacket(packet.(*ProtoPacket))
		}
	}
}

// 心跳回复
func onHeartBeatReq(connection Connection, packet Packet) {
	req := packet.Message().(*pb.HeartBeatReq)
	SendPacketAdapt(connection, packet, PacketCommand(pb.CmdInner_Cmd_HeartBeatRes), &pb.HeartBeatRes{
		RequestTimestamp:  req.GetTimestamp(),
		ResponseTimestamp: uint64(time.Now().UnixNano() / int64(time.Microsecond)),
	})
}

// 注册网关消息回调
func (this *GameServer) registerGatePacket(gateHandler PacketHandlerRegister) {
	// 手动注册消息回调
	gateHandler.Register(PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), onHeartBeatReq, new(pb.HeartBeatReq))
	gateHandler.Register(PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameReq), onPlayerEntryGameReq, new(pb.PlayerEntryGameReq))
	gateHandler.Register(PacketCommand(pb.CmdLogin_Cmd_CreatePlayerReq), onCreatePlayerReq, new(pb.CreatePlayerReq))
	gateHandler.Register(PacketCommand(pb.CmdInner_Cmd_ClientDisconnect), onClientDisconnect, new(pb.TestCmd))
	gateHandler.Register(PacketCommand(pb.CmdInner_Cmd_TestCmd), wrapPlayerHandler(onTestCmd), new(pb.TestCmd))
	gateHandler.(*DefaultConnectionHandler).SetUnRegisterHandler(func(connection Connection, packet Packet) {
		if gatePacket,ok := packet.(*GatePacket); ok {
			playerId := gatePacket.PlayerId()
			player := this.GetPlayer(playerId)
			if player != nil {
				if gamePlayer ,ok := player.(*game.Player); ok {
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
	gateHandler.(*DefaultConnectionHandler).SetOnDisconnectedFunc(func(connection Connection) {
		this.playerMap.Range(func(key, value interface{}) bool {
			if player,ok := value.(*game.Player); ok {
				player.OnDisconnect(connection)
			}
			return true
		})
	})
	// 通过反射自动注册消息和proto.Message的映射
	game.AutoRegisterPlayerComponentProto(gateHandler)
	// 自动注册消息回调的另一种方案: proto_code_gen工具生成的回调函数
	// 因为已经用了反射自动注册,所以这里注释了
	// player_component_handler_gen(clientHandler)
}

// 注册服务器消息回调
func (this *GameServer) registerServerPacket(serverHandler *DefaultConnectionHandler) {
	serverHandler.Register(PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), onHeartBeatReq, new(pb.HeartBeatReq))
	serverHandler.Register(PacketCommand(pb.CmdInner_Cmd_KickPlayer), this.onKickPlayer, new(pb.KickPlayer))
	serverHandler.Register(PacketCommand(pb.CmdRoute_Cmd_RoutePlayerMessage), this.onRoutePlayerMessage, new(pb.RoutePlayerMessage))
	//serverHandler.autoRegisterPlayerComponentProto()
}

// 添加一个在线玩家
func (this *GameServer) AddPlayer(player gentity.Player) {
	this.playerMap.Store(player.GetId(), player)
	cache.AddOnlinePlayer(player.GetId(), player.GetAccountId(), this.GetId())
}

// 删除一个在线玩家
func (this *GameServer) RemovePlayer(player gentity.Player) {
	// 先保存数据库 再移除cache
	player.(*game.Player).SaveDb(true)
	this.playerMap.Delete(player.GetId())
	cache.RemoveOnlineAccount(player.GetAccountId())
	cache.RemoveOnlinePlayer(player.GetId(), this.GetId())
}

// 获取一个在线玩家
func (this *GameServer) GetPlayer(playerId int64) gentity.Player {
	if v, ok := this.playerMap.Load(playerId); ok {
		return v.(gentity.Player)
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
		logger.Error("kick player failed account:%v playerId:%v gameServerId:%v",
			req.GetAccountId(), req.GetPlayerId(), this.GetId())
	}
}

// 转发玩家消息
// otherServer -> thisServer -> player
func (this *GameServer) onRoutePlayerMessage(connection Connection, packet Packet) {
	req := packet.Message().(*pb.RoutePlayerMessage)
	logger.Debug("onRoutePlayerMessage %v", req)
	player := game.GetPlayer(req.ToPlayerId)
	if player == nil {
		// NOTE: 由于是异步消息,这里的player有很低的概率可能不在线了,如果是重要的不能丢弃的消息,需要保存该消息,留待后续处理
		// 演示程序暂不处理,这里就直接丢弃了
		logger.Error("player nil %v", req.ToPlayerId)
		return
	}
	message, err := req.PacketData.UnmarshalNew()
	if err != nil {
		logger.Error("UnmarshalNew %v err:%v", req.ToPlayerId, err)
		return
	}
	err = req.PacketData.UnmarshalTo(message)
	if err != nil {
		logger.Error("UnmarshalTo %v err:%v", req.ToPlayerId, err)
		return
	}
	if req.DirectSendClient {
		// 不需要player处理的消息,直接转发给客户端
		player.Send(PacketCommand(uint16(req.PacketCommand)), message)
	} else {
		// 需要player处理的消息,放进player的消息队列,在玩家的逻辑协程中处理
		player.OnRecvPacket(NewProtoPacket(PacketCommand(req.PacketCommand), message))
	}
}
