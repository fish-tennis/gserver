package game

import (
	"context"
	"encoding/json"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/db/mongodb"
	"github.com/fish-tennis/gserver/gameplayer"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/misc"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/social"
	"os"
	"sync"
	"time"
)

var (
	_ Server = (*GameServer)(nil)
)

// 游戏服
type GameServer struct {
	BaseServer
	config *GameServerConfig
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
	gameplayer.SetPlayerMgr(this)
	if !this.BaseServer.Init(ctx, configFile) {
		return false
	}
	this.readConfig()
	gameplayer.InitPlayerComponentMap()
	this.initDb()
	this.initCache()

	netMgr := GetNetMgr()
	// 客户端的codec和handler
	clientCodec := NewProtoCodec(nil)
	clientHandler := gameplayer.NewClientConnectionHandler(clientCodec)
	this.registerClientPacket(clientHandler)

	// 服务器的codec和handler
	serverCodec := NewProtoCodec(nil)
	serverHandler := NewDefaultConnectionHandler(serverCodec)

	if netMgr.NewListener(ctx, this.config.ClientListenAddr, this.config.ClientConnConfig, clientCodec,
		clientHandler, &ClientListerHandler{}) == nil {
		panic("listen client failed")
		return false
	}

	this.registerServerPacket(serverHandler)
	this.serverListener = netMgr.NewListener(ctx, this.config.ServerListenAddr, this.config.ServerConnConfig, serverCodec,
		serverHandler, nil)
	if this.serverListener == nil {
		panic("listen server failed")
		return false
	}

	// 连接其他服务器
	this.BaseServer.SetDefaultServerConnectorConfig(this.config.ServerConnConfig)
	this.BaseServer.GetServerList().SetFetchAndConnectServerTypes("game")

	// 其他模块初始化
	this.AddServerHook(&social.Hook{})
	serverInitArg := &misc.GameServerInitArg{
		ClientHandler: clientHandler,
		ServerHandler: serverHandler,
		PlayerMgr: gameplayer.GetPlayerMgr(),
	}
	for _,hook := range this.BaseServer.GetServerHooks() {
		hook.OnServerInit(serverInitArg)
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
		player := value.(*gameplayer.Player)
		player.Stop()
		return true
	})
	this.BaseServer.Exit()
	logger.Info("GameServer.Exit")
	dbMgr := db.GetDbMgr()
	if dbMgr != nil {
		dbMgr.(*mongodb.MongoDb).Disconnect()
	}
}

// 读取配置文件
func (this *GameServer) readConfig() {
	fileData,err := os.ReadFile(this.GetConfigFile())
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
	this.BaseServer.GetServerInfo().ServerType = "game"
	this.BaseServer.GetServerInfo().ClientListenAddr = this.config.ClientListenAddr
	this.BaseServer.GetServerInfo().ServerListenAddr = this.config.ServerListenAddr
}

// 初始化数据库
func (this *GameServer) initDb() {
	// 使用mongodb来演示
	mongoDb := mongodb.NewMongoDb(this.config.MongoUri,"testdb")
	mongoDb.RegisterPlayerPb("player", "id", "name", "accountid", "regionid")
	mongoDb.RegisterEntityDb("guild", "id", "name")
	//mongoDb.SetAccountColumnNames("accountid","")
	//mongoDb.SetPlayerColumnNames("id", "name","regionid")
	if !mongoDb.Connect() {
		panic("connect db error")
	}
	db.SetDbMgr(mongoDb)
}

// 初始化redis缓存
func (this *GameServer) initCache() {
	cache.NewRedis(this.config.RedisUri, this.config.RedisPassword, this.config.RedisCluster)
	pong,err := cache.GetRedis().Ping(context.Background()).Result()
	if err != nil || pong == "" {
		panic("redis connect error")
	}
	this.repairCache()
}

// 修复缓存,游戏服异常宕机重启后进行修复操作
func (this *GameServer) repairCache() {
	cache.ResetOnlinePlayer(this.GetServerId(), this.repairPlayerCache)
}

// 缓存中的玩家数据保存到数据库
// 服务器crash时,缓存数据没来得及保存到数据库,服务器重启后进行自动修复,防止玩家数据回档
func (this *GameServer) repairPlayerCache(playerId,accountId int64) error {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("repairPlayerCache %v err:%v", playerId, err.(error).Error())
			LogStack()
		}
	}()
	tmpPlayer := gameplayer.CreateTempPlayer(playerId,accountId)
	for _,component := range tmpPlayer.GetComponents() {
		if saveable,ok := component.(Saveable); ok {
			err := LoadFromCache(saveable)
			if err != nil {
				logger.Error("LoadFromCache %v error:%v", saveable.GetCacheKey(), err.Error())
				continue
			}
			saveData,err := SaveSaveable(saveable)
			if err != nil {
				logger.Error("%v Save %v err %v", playerId, component.GetName(), err.Error())
				continue
			}
			saveDbErr := db.GetPlayerDb().SaveComponent(playerId, component.GetNameLower(), saveData)
			if saveDbErr != nil {
				logger.Error("%v SaveDb %v err %v", playerId, component.GetNameLower(), saveDbErr.Error())
				continue
			}
			logger.Info("%v SaveDb %v", playerId, component.GetNameLower())
			cache.Get().Del(saveable.GetCacheKey())
			logger.Info("RemoveCache %v %v", playerId, saveable.GetCacheKey())
		}
		if compositeSaveable,ok := component.(CompositeSaveable); ok {
			saveables := compositeSaveable.SaveableChildren()
			for _,saveable := range saveables {
				err := LoadFromCache(saveable)
				if err != nil {
					logger.Error("LoadFromCache %v error:%v", saveable.GetCacheKey(), err.Error())
					continue
				}
				saveData,err := SaveSaveable(saveable)
				if err != nil {
					logger.Error("Save %v err:%v", saveable.GetCacheKey(), err.Error())
					continue
				}
				if saveData == nil {
					logger.Info("ignore nil %v", saveable.GetCacheKey())
					continue
				}
				saveDbErr := db.GetPlayerDb().SaveComponentField(playerId, component.GetNameLower(), saveable.Key(), saveData)
				if saveDbErr != nil {
					logger.Error("%v SaveDb %v.%v err %v", playerId, component.GetNameLower(), saveable.Key(), saveDbErr.Error())
					continue
				}
				logger.Info("%v SaveDb %v", playerId, component.GetNameLower()+"."+saveable.Key())
				cache.Get().Del(saveable.GetCacheKey())
				logger.Info("RemoveCache %v %v", playerId, saveable.GetCacheKey())
			}
		}
	}
	return nil
}

// 注册客户端消息回调
func (this *GameServer) registerClientPacket(clientHandler *gameplayer.ClientConnectionHandler) {
	// 手动注册消息回调
	clientHandler.Register(PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), onHeartBeatReq, new(pb.HeartBeatReq))
	clientHandler.Register(PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameReq), onPlayerEntryGameReq, new(pb.PlayerEntryGameReq))
	clientHandler.Register(PacketCommand(pb.CmdLogin_Cmd_CreatePlayerReq), onCreatePlayerReq, new(pb.CreatePlayerReq))
	// 通过反射自动注册消息回调
	//clientHandler.autoRegisterPlayerComponentProto()
	gameplayer.AutoRegisterPlayerComponentProto(clientHandler)
	// 自动注册消息回调的另一种方案: proto_code_gen工具生成的回调函数
	// 因为已经用了反射自动注册,所以这里注释了
	// player_component_handler_auto_register(clientHandler)
}

// 心跳回复
func onHeartBeatReq(connection Connection, packet *ProtoPacket) {
	req := packet.Message().(*pb.HeartBeatReq)
	connection.Send(PacketCommand(pb.CmdInner_Cmd_HeartBeatRes), &pb.HeartBeatRes{
		RequestTimestamp: req.GetTimestamp(),
		ResponseTimestamp: uint64(time.Now().UnixNano()/int64(time.Microsecond)),
	})
}

// 注册服务器消息回调
func (this *GameServer) registerServerPacket(serverHandler *DefaultConnectionHandler) {
	serverHandler.Register(PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), onHeartBeatReq, new(pb.HeartBeatReq))
	serverHandler.Register(PacketCommand(pb.CmdInner_Cmd_KickPlayer), this.onKickPlayer, new(pb.KickPlayer))
	serverHandler.Register(PacketCommand(pb.CmdRoute_Cmd_RoutePlayerMessage), this.onRoutePlayerMessage, new(pb.RoutePlayerMessage))
	//serverHandler.autoRegisterPlayerComponentProto()
}

// 添加一个在线玩家
func (this *GameServer) AddPlayer(player *gameplayer.Player) {
	this.playerMap.Store(player.GetId(), player)
	cache.AddOnlinePlayer(player.GetId(), player.GetAccountId(), this.GetServerId())
}

// 删除一个在线玩家
func (this *GameServer) RemovePlayer(player *gameplayer.Player) {
	// 先保存数据库 再移除cache
	player.SaveDb(true)
	this.playerMap.Delete(player.GetId())
	cache.RemoveOnlineAccount(player.GetAccountId())
	cache.RemoveOnlinePlayer(player.GetId(), this.GetServerId())
}

// 获取一个在线玩家
func (this *GameServer) GetPlayer(playerId int64) *gameplayer.Player {
	if v,ok := this.playerMap.Load(playerId); ok {
		return v.(*gameplayer.Player)
	}
	return nil
}

// 踢玩家下线
func (this *GameServer) onKickPlayer(connection Connection, packet *ProtoPacket) {
	req := packet.Message().(*pb.KickPlayer)
	player := this.GetPlayer(req.GetPlayerId())
	if player != nil {
		player.SetConnection(nil)
		player.Stop()
	} else {
		logger.Error("kick player failed account:%v playerId:%v gameServerId:%v",
			req.GetAccountId(), req.GetPlayerId(), this.GetServerId())
	}
}

// 转发玩家消息
// otherServer -> thisServer -> client
func (this *GameServer) onRoutePlayerMessage(connection Connection, packet *ProtoPacket) {
	logger.Debug("onRoutePlayerMessage")
	req := packet.Message().(*pb.RoutePlayerMessage)
	player := this.GetPlayer(req.ToPlayerId)
	if player == nil {
		logger.Error("player nil %v", req.ToPlayerId)
		return
	}
	message,err := req.PacketData.UnmarshalNew()
	if err != nil {
		logger.Error("UnmarshalNew %v err:%v", req.ToPlayerId, err)
		return
	}
	err = req.PacketData.UnmarshalTo(message)
	if err != nil {
		logger.Error("UnmarshalTo %v err:%v", req.ToPlayerId, err)
		return
	}
	player.Send(PacketCommand(uint16(req.PacketCommand)), message)
}