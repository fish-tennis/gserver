package game

import (
	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"reflect"
	"time"
)

const (
	// Player在redis里的前缀
	PlayerCachePrefix = "p"
)

var _ gentity.RoutineEntity = (*Player)(nil)

// 玩家对象
type Player struct {
	gentity.BaseRoutineEntity
	// 玩家名
	name string
	// 账号id
	accountId int64
	// 区服id
	regionId int32
	//accountName string
	// 是否使用网关
	useGate bool
	// 关联的连接,如果是网关模式,就是网关的连接
	// 如果是客户端直连模式,就是客户端连接
	connection Connection
	// 事件分发的嵌套检测
	fireEventLoopChecker map[reflect.Type]int32
	// 进度更新
	progressEventMapping *ProgressEventMapping
}

// 玩家名(unique)
func (p *Player) GetName() string {
	return p.name
}

// 账号id
func (p *Player) GetAccountId() int64 {
	return p.accountId
}

// 区服id
func (p *Player) GetRegionId() int32 {
	return p.regionId
}

// 玩家数据保存数据库
func (p *Player) SaveDb(removeCacheAfterSaveDb bool) error {
	return gentity.SaveEntityChangedDataToDb(db.GetPlayerDb(), p, cache.Get(), removeCacheAfterSaveDb, PlayerCachePrefix)
}

func (p *Player) SaveCache(kvCache gentity.KvCache) error {
	return p.BaseEntity.SaveCache(kvCache, PlayerCachePrefix, p.GetId())
}

// 设置关联的连接,支持客户端直连模式和网关模式
func (p *Player) SetConnection(connection Connection, useGate bool) {
	p.useGate = useGate
	if !useGate {
		// 取消之前的连接和该玩家的关联
		if p.connection != nil && p.connection != connection {
			p.connection.SetTag(nil)
		}
		// 客户端直连模式,设置连接和玩家的关联
		if connection != nil {
			connection.SetTag(p.GetId())
		}
	}
	p.connection = connection
}

func (p *Player) ResetConnection() {
	p.connection = nil
}

func (p *Player) GetConnection() Connection {
	return p.connection
}

func (p *Player) OnDisconnect(connection Connection) {
	if p.GetConnection() == connection {
		p.ResetConnection()
		p.Stop()
		logger.Debug("player %v exit", p.GetId())
	}
}

// 发包(protobuf)
// NOTE:调用Send(message)之后,不要再对message进行读写!
func (p *Player) Send(message proto.Message, opts ...SendOption) bool {
	clientCmd := network.GetCommandByProto(message)
	if clientCmd <= 0 {
		slog.Error("clientCmdNotFound", "messageName", proto.MessageName(message))
		return false
	}
	return p.SendWithCommand(PacketCommand(clientCmd), message, opts...)
}

func (p *Player) SendWithCommand(cmd PacketCommand, message proto.Message, opts ...SendOption) bool {
	if p.connection != nil {
		if p.useGate {
			// 网关模式,自动附加上playerId
			return p.connection.SendPacket(network.NewGatePacket(p.GetId(), cmd, message), opts...)
		} else {
			return p.connection.Send(cmd, message, opts...)
		}
	}
	return false
}

func (p *Player) SendPacket(packet Packet, opts ...SendOption) bool {
	if p.connection != nil {
		if p.useGate {
			// 网关模式,自动附加上playerId
			return p.connection.SendPacket(network.NewGatePacket(p.GetId(), packet.Command(), packet.Message()).
				WithStreamData(packet.GetStreamData()).WithRpc(packet), opts...)
		} else {
			return p.connection.SendPacket(packet, opts...)
		}
	}
	return false
}

// 通用的错误返回消息
func (p *Player) SendErrorRes(errorReqCmd PacketCommand, errorMsg string) bool {
	return p.SendWithCommand(PacketCommand(pb.CmdClient_Cmd_ErrorRes), &pb.ErrorRes{
		Command:   int32(errorReqCmd),
		ResultStr: errorMsg,
	})
}

// 分发事件
func (p *Player) FireEvent(event any) {
	slog.Debug("FireEvent", "pid", p.GetId(), "event", event)
	// 嵌套检测
	if p.fireEventLoopChecker == nil {
		p.fireEventLoopChecker = make(map[reflect.Type]int32)
	}
	eventTyp := reflect.TypeOf(event)
	p.fireEventLoopChecker[eventTyp]++
	defer func() {
		p.fireEventLoopChecker[eventTyp]--
		if p.fireEventLoopChecker[eventTyp] <= 0 {
			delete(p.fireEventLoopChecker, eventTyp)
		}
	}()
	curLoopCount := p.fireEventLoopChecker[eventTyp]
	if curLoopCount > 1 {
		slog.Debug("FireEventLoopChecker", "loop", curLoopCount)
		if curLoopCount > internal.SameEventLoopLimit {
			slog.Error("FireEvent limit", "loop", curLoopCount)
			// 防止事件分发的嵌套导致死循环
			return
		}
	}
	// 注册的事件响应接口
	_playerEventHandlerMgr.Invoke(p, event)
	// 有些模块有通用的处理接口
	p.RangeEventReceiver(func(eventReceiver gentity.EventReceiver) bool {
		eventReceiver.OnEvent(event)
		return true
	})
	// 进度更新
	p.progressEventMapping.OnTriggerEvent(event)
}

// 分发条件相关事件
func (p *Player) FireConditionEvent(event interface{}) {
	logger.Debug("%v FireConditionEvent:%v", p.GetId(), event)
	// 进度更新
	p.progressEventMapping.OnTriggerEvent(event)
}

// 分发事件,但是延后执行
func (p *Player) PostEvent(event any) {
	// TODO: 先保存起来,再延后执行
}

func (p *Player) GetLevel() int32 {
	return p.GetBaseInfo().Data.Level
}

// 开启消息处理协程
// 每个玩家一个独立的消息处理协程
// 除了登录消息,其他消息都在玩家自己的协程里处理,因此这里对本玩家的操作不需要加锁
func (p *Player) RunRoutine() bool {
	logger.Debug("player RunRoutine %v", p.GetId())
	ok := p.RunProcessRoutine(p, &gentity.RoutineEntityRoutineArgs{
		EndFunc: func(routineEntity gentity.RoutineEntity) {
			// 分发事件:玩家退出游戏
			p.FireEvent(&internal.EventPlayerExit{})
			// 协程结束的时候,移除玩家
			GetPlayerMgr().RemovePlayer(p)
		},
		ProcessMessageFunc: func(routineEntity gentity.RoutineEntity, message any) {
			p.processMessage(message.(*ProtoPacket))
		},
		AfterTimerExecuteFunc: func(routineEntity gentity.RoutineEntity, t time.Time) {
			// 如果有需要保存的数据修改了,即时保存数据库
			p.SaveCache(cache.Get())
		},
	})
	if ok {
		// 每分钟执行一次,刷新在线时间
		p.GetTimerEntries().After(time.Minute, func() time.Duration {
			evt := &pb.EventPlayerProperty{
				PlayerId: p.GetId(),
				Property: "OnlineMinute",
				Delta:    1,
				Current:  p.GetPropertyInt32("OnlineMinute"),
			}
			p.FireEvent(evt)
			return time.Minute
		})
	}
	return ok
}

func (p *Player) processMessage(message *ProtoPacket) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("recover:%v", err)
			logger.LogStack()
		}
	}()
	logger.Debug("processMessage %v", proto.MessageName(message.Message()).Name())
	// func (c *Component) OnXxxReq(req *pb.XxxReq)
	// func (c *Component) OnXxxReq(req *pb.XxxReq) (*pb.XxxRes,error)
	if _playerPacketHandlerMgr.Invoke(p, message, func(handlerInfo *internal.PacketHandlerInfo, returnValues []reflect.Value) {
		if handlerInfo.ResCmd == 0 || len(returnValues) != 2 {
			return
		}
		resProto, _ := returnValues[0].Interface().(proto.Message)
		resErr, _ := returnValues[1].Interface().(error)
		if resProto == nil {
			resProto = reflect.New(handlerInfo.ResMessageElem).Interface().(proto.Message)
		}
		// 返回消息给客户端
		if resErr != nil {
			p.SendErrorRes(handlerInfo.ResCmd, resErr.Error())
		} else {
			p.Send(resProto)
		}
	}) {
		// 如果有需要保存的数据修改了,即时保存缓存
		p.SaveCache(cache.Get())
		return
	}
	logger.Error("unhandled cmd:%v message:%v", message.Command(), proto.MessageName(message.Message()).Name())
}

// 放入消息队列
func (p *Player) OnRecvPacket(packet *ProtoPacket) {
	logger.Debug("OnRecvPacket %v", proto.MessageName(packet.Message()).Name())
	p.PushMessage(packet)
}

// 玩家进入游戏服
func (p *Player) HandlePlayerEntryGameOk(msg *pb.PlayerEntryGameOk) {
	slog.Debug("HandlePlayerEntryGameOk", "pid", p.GetId(), "msg", msg)
	// 同步各模块的数据给客户端
	p.RangeComponent(func(component gentity.Component) bool {
		if dataSyncer, ok := component.(DataSyncer); ok {
			dataSyncer.SyncDataToClient()
			slog.Debug("SyncDataToClient", "pid", p.GetId(), "component", component.GetName())
		}
		return true
	})
	b := p.GetBaseInfo()
	now := p.GetTimerEntries().Now().Unix()
	var offlineSeconds int32
	if b.Data.LastLogoutTimestamp > 0 && now > b.Data.LastLogoutTimestamp {
		offlineSeconds = int32(now - b.Data.LastLogoutTimestamp)
	}
	b.Data.LastLoginTimestamp = now
	b.SetDirty()
	// 分发事件:玩家进游戏服
	p.FireEvent(&internal.EventPlayerEntryGame{
		IsReconnect:    msg.IsReconnect,
		OfflineSeconds: offlineSeconds,
	})
}

func CreatePlayer(playerId int64, playerName string, accountId int64, regionId int32) *Player {
	player := &Player{
		name:              playerName,
		accountId:         accountId,
		regionId:          regionId,
		BaseRoutineEntity: *gentity.NewRoutineEntity(32),
	}
	player.Id = playerId
	player.progressEventMapping = &ProgressEventMapping{
		player: player,
	}
	// 初始化玩家的各个模块
	_playerComponentRegister.InitComponents(player, nil)
	return player
}

// 从加载的数据构造出玩家对象
func CreatePlayerFromData(playerData *pb.PlayerData) *Player {
	var player *Player
	defer func() {
		if err := recover(); err != nil {
			player = nil
			logger.Error("fatal %v", err.(error))
			LogStack()
		}
	}()
	player = CreatePlayer(playerData.XId, playerData.Name, playerData.AccountId, playerData.RegionId)
	err := gentity.LoadEntityData(player, playerData)
	if err != nil {
		slog.Error("LoadPlayerDataErr", "playerId", player.GetId(), "err", err)
		return nil
	}
	player.RangeComponent(func(component gentity.Component) bool {
		if dataLoader, ok := component.(internal.DataLoader); ok {
			dataLoader.OnDataLoad()
			slog.Debug("OnDataLoad", "pid", player.GetId(), "component", component.GetName())
		}
		return true
	})
	return player
}

func CreateTempPlayer(playerId, accountId int64) *Player {
	return CreatePlayer(playerId, "", accountId, 0)
}
