package game

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/internal"
	"time"

	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
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

// 获取组件
func (this *Player) GetComponent(componentName string) gentity.Component {
	index := GetComponentIndex(componentName)
	if index >= 0 {
		return this.GetComponentByIndex(index)
	}
	return nil
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
func (this *Player) Send(command PacketCommand, message proto.Message, opts ...SendOption) bool {
	if this.connection != nil {
		if this.useGate {
			// 网关模式,自动附加上playerId
			return this.connection.SendPacket(internal.NewGatePacket(this.GetId(), command, message), opts...)
		} else {
			return this.connection.Send(command, message, opts...)
		}
	}
	return false
}

func (this *Player) SendPacket(packet Packet, opts ...SendOption) bool {
	if this.connection != nil {
		if this.useGate {
			// 网关模式,自动附加上playerId
			return this.connection.SendPacket(internal.NewGatePacket(this.GetId(), packet.Command(), packet.Message()).
				WithStreamData(packet.GetStreamData()).WithRpc(packet), opts...)
		} else {
			return this.connection.SendPacket(packet, opts...)
		}
	}
	return false
}

// 通用的错误返回消息
func (this *Player) SendErrorRes(errorReqCmd PacketCommand, errorMsg string) bool {
	return this.Send(PacketCommand(pb.CmdInner_Cmd_ErrorRes), &pb.ErrorRes{
		Command:   int32(errorReqCmd),
		ResultStr: errorMsg,
	})
}

// 分发事件给组件
func (this *Player) FireEvent(event interface{}) {
	logger.Debug("%v FireEvent:%v", this.GetId(), event)
	// TODO:建一个事件类型和组件的映射表 eventType -> component list
	this.RangeComponent(func(component gentity.Component) bool {
		if eventReceiver, ok := component.(gentity.EventReceiver); ok {
			eventReceiver.OnEvent(event)
		}
		return true
	})
}

// 分发条件相关事件
func (this *Player) FireConditionEvent(event interface{}) {
	logger.Debug("%v FireConditionEvent:%v", this.GetId(), event)
	this.GetQuest().Quests.OnEvent(event)
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
	// 先找组件接口
	if _playerComponentHandlerRegister.Invoke(this, message) {
		// 如果有需要保存的数据修改了,即时保存缓存
		this.SaveCache(cache.Get())
		return
	}
	// 再找func(player *Player, packet Packet)格式的回调接口
	if playerHandler, ok := _playerHandlerRegister[message.Command()]; ok {
		playerHandler(this, message)
		this.SaveCache(cache.Get())
		return
	}
	// 再找func(connection Connection, packet Packet)的回调接口
	packetHandler := _clientConnectionHandler.GetPacketHandler(message.Command())
	if packetHandler != nil {
		packetHandler(this.GetConnection(), message)
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

// 从加载的数据构造出玩家对象
func CreatePlayerFromData(playerData *pb.PlayerData) *Player {
	player := &Player{
		name:              playerData.Name,
		accountId:         playerData.AccountId,
		regionId:          playerData.RegionId,
		BaseRoutineEntity: *gentity.NewRoutineEntity(32),
	}
	player.Id = playerData.XId
	// 初始化玩家的各个模块
	_playerComponentRegister.InitComponents(player, playerData)
	return player
}

func CreateTempPlayer(playerId, accountId int64) *Player {
	playerData := &pb.PlayerData{}
	player := CreatePlayerFromData(playerData)
	player.Id = playerId
	player.accountId = accountId
	return player
}

func NewEmptyPlayer(playerId int64) *Player {
	p := &Player{}
	p.Id = playerId
	return p
}
