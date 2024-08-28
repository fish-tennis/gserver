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
}

// 玩家名(unique)
func (this *Player) GetName() string {
	return this.name
}

// 账号id
func (this *Player) GetAccountId() int64 {
	return this.accountId
}

// 区服id
func (this *Player) GetRegionId() int32 {
	return this.regionId
}

// 玩家数据保存数据库
func (this *Player) SaveDb(removeCacheAfterSaveDb bool) error {
	return gentity.SaveEntityChangedDataToDb(db.GetPlayerDb(), this, cache.Get(), removeCacheAfterSaveDb, PlayerCachePrefix)
}

func (this *Player) SaveCache(kvCache gentity.KvCache) error {
	return this.BaseEntity.SaveCache(kvCache, PlayerCachePrefix, this.GetId())
}

// 设置关联的连接,支持客户端直连模式和网关模式
func (this *Player) SetConnection(connection Connection, useGate bool) {
	this.useGate = useGate
	if !useGate {
		// 取消之前的连接和该玩家的关联
		if this.connection != nil && this.connection != connection {
			this.connection.SetTag(nil)
		}
		// 客户端直连模式,设置连接和玩家的关联
		if connection != nil {
			connection.SetTag(this.GetId())
		}
	}
	this.connection = connection
}

func (this *Player) ResetConnection() {
	this.connection = nil
}

func (this *Player) GetConnection() Connection {
	return this.connection
}

func (this *Player) OnDisconnect(connection Connection) {
	if this.GetConnection() == connection {
		this.ResetConnection()
		this.Stop()
		logger.Debug("player %v exit", this.GetId())
	}
}

// 发包(protobuf)
// NOTE:调用Send(command,message)之后,不要再对message进行读写!
func (this *Player) Send(message proto.Message, opts ...SendOption) bool {
	clientCmd := internal.GetClientCommandByProto(message)
	if clientCmd <= 0 {
		slog.Error("clientCmdNotFound", "messageName", proto.MessageName(message))
		return false
	}
	return this.SendWithCommand(PacketCommand(clientCmd), message, opts...)
}

func (this *Player) SendWithCommand(cmd PacketCommand, message proto.Message, opts ...SendOption) bool {
	if this.connection != nil {
		if this.useGate {
			// 网关模式,自动附加上playerId
			return this.connection.SendPacket(network.NewGatePacket(this.GetId(), PacketCommand(cmd), message), opts...)
		} else {
			return this.connection.Send(PacketCommand(cmd), message, opts...)
		}
	}
	return false
}

func (this *Player) SendPacket(packet Packet, opts ...SendOption) bool {
	if this.connection != nil {
		if this.useGate {
			// 网关模式,自动附加上playerId
			return this.connection.SendPacket(network.NewGatePacket(this.GetId(), packet.Command(), packet.Message()).
				WithStreamData(packet.GetStreamData()).WithRpc(packet), opts...)
		} else {
			return this.connection.SendPacket(packet, opts...)
		}
	}
	return false
}

// 通用的错误返回消息
func (this *Player) SendErrorRes(errorReqCmd PacketCommand, errorMsg string) bool {
	return this.SendWithCommand(PacketCommand(pb.CmdInner_Cmd_ErrorRes), &pb.ErrorRes{
		Command:   int32(errorReqCmd),
		ResultStr: errorMsg,
	})
}

// 分发事件
func (this *Player) FireEvent(event any) {
	// 嵌套检测
	if this.fireEventLoopChecker == nil {
		this.fireEventLoopChecker = make(map[reflect.Type]int32)
	}
	eventTyp := reflect.TypeOf(event)
	this.fireEventLoopChecker[eventTyp]++
	defer func() {
		this.fireEventLoopChecker[eventTyp]--
		if this.fireEventLoopChecker[eventTyp] <= 0 {
			delete(this.fireEventLoopChecker, eventTyp)
		}
	}()
	curLoopCount := this.fireEventLoopChecker[eventTyp]
	if curLoopCount > 1 {
		slog.Debug("FireEventLoopChecker", "loop", curLoopCount)
		if curLoopCount > internal.SameEventLoopLimit {
			slog.Error("FireEvent limit", "loop", curLoopCount)
			// 防止事件分发的嵌套导致死循环
			return
		}
	}
	// 注册的事件响应接口
	_playerEventHandlerMgr.Invoke(this, event)
	// 有些模块有通用的处理接口
	this.RangeEventReceiver(func(eventReceiver gentity.EventReceiver) bool {
		eventReceiver.OnEvent(event)
		return true
	})
}

// 分发条件相关事件
func (this *Player) FireConditionEvent(event interface{}) {
	logger.Debug("%v FireConditionEvent:%v", this.GetId(), event)
	this.GetQuest().OnEvent(event)
	this.GetActivities().OnEvent(event)
}

func (this *Player) GetLevel() int32 {
	return this.GetBaseInfo().Data.Level
}

// 开启消息处理协程
// 每个玩家一个独立的消息处理协程
// 除了登录消息,其他消息都在玩家自己的协程里处理,因此这里对本玩家的操作不需要加锁
func (this *Player) RunRoutine() bool {
	logger.Debug("player RunRoutine %v", this.GetId())
	ok := this.RunProcessRoutine(this, &gentity.RoutineEntityRoutineArgs{
		EndFunc: func(routineEntity gentity.RoutineEntity) {
			// 分发事件:玩家退出游戏
			this.FireEvent(&internal.EventPlayerExit{})
			// 协程结束的时候,移除玩家
			GetPlayerMgr().RemovePlayer(this)
		},
		ProcessMessageFunc: func(routineEntity gentity.RoutineEntity, message any) {
			this.processMessage(message.(*ProtoPacket))
		},
		AfterTimerExecuteFunc: func(routineEntity gentity.RoutineEntity, t time.Time) {
			// 如果有需要保存的数据修改了,即时保存数据库
			this.SaveCache(cache.Get())
		},
	})
	if ok {
		// 每分钟执行一次,刷新在线时间
		this.GetTimerEntries().After(time.Minute, func() time.Duration {
			evt := &pb.EventPlayerPropertyInc{
				PlayerId:      this.GetId(),
				PropertyName:  "OnlineMinute",
				PropertyValue: 1,
			}
			this.FireEvent(evt)
			return time.Minute
		})
	}
	return ok
}

func (this *Player) processMessage(message *ProtoPacket) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("recover:%v", err)
			logger.LogStack()
		}
	}()
	logger.Debug("processMessage %v", proto.MessageName(message.Message()).Name())
	// func (c *Component) OnXxxReq(req *pb.XxxReq)
	// func (c *Component) OnXxxReq(req *pb.XxxReq) (*pb.XxxRes,error)
	if _playerPacketHandlerMgr.Invoke(this, message, func(handlerInfo *internal.PacketHandlerInfo, returnValues []reflect.Value) {
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
			this.SendErrorRes(handlerInfo.ResCmd, resErr.Error())
		} else {
			this.Send(resProto)
		}
	}) {
		// 如果有需要保存的数据修改了,即时保存缓存
		this.SaveCache(cache.Get())
		return
	}
	logger.Error("unhandled cmd:%v message:%v", message.Command(), proto.MessageName(message.Message()).Name())
}

// 放入消息队列
func (this *Player) OnRecvPacket(packet *ProtoPacket) {
	logger.Debug("OnRecvPacket %v", proto.MessageName(packet.Message()).Name())
	this.PushMessage(packet)
}

func CreatePlayer(playerId int64, playerName string, accountId int64, regionId int32) *Player {
	player := &Player{
		name:              playerName,
		accountId:         accountId,
		regionId:          regionId,
		BaseRoutineEntity: *gentity.NewRoutineEntity(32),
	}
	player.Id = playerId
	// 初始化玩家的各个模块
	_playerComponentRegister.InitComponents(player, nil)
	return player
}

// 从加载的数据构造出玩家对象
func CreatePlayerFromData(playerData *pb.PlayerData) *Player {
	player := CreatePlayer(playerData.XId, playerData.Name, playerData.AccountId, playerData.RegionId)
	err := gentity.LoadEntityData(player, playerData)
	if err != nil {
		slog.Error("LoadPlayerDataErr", "playerId", player.GetId(), "err", err)
		return nil
	}
	return player
}

func CreateTempPlayer(playerId, accountId int64) *Player {
	return CreatePlayer(playerId, "", accountId, 0)
}
