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
	return gentity.SaveEntityChangedDataToDb(db.GetPlayerDb(), this, cache.Get(), removeCacheAfterSaveDb)
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

func (this *Player) GetConnection() Connection {
	return this.connection
}

// 发包(protobuf)
// NOTE:调用Send(command,message)之后,不要再对message进行读写!
func (this *Player) Send(command PacketCommand, message proto.Message) bool {
	if this.connection != nil {
		if this.useGate {
			// 网关模式,自动附加上playerId
			return this.connection.SendPacket(internal.NewGatePacket(this.GetId(), command, message))
		} else {
			return this.connection.Send(command, message)
		}
	}
	return false
}

// 通用的错误返回消息
func (this *Player) SendErrorRes(errorReqCmd PacketCommand, errorMsg string) bool {
	if this.connection != nil {
		return this.connection.Send(PacketCommand(pb.CmdInner_Cmd_ErrorRes), &pb.ErrorRes{
			Command:   int32(errorReqCmd),
			ResultStr: errorMsg,
		})
	}
	return false
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
	// 目前只有任务模块用了Condition
	this.GetQuest().Quests.fireEvent(event)
}

func (this *Player) GetBaseInfo() *BaseInfo {
	return this.GetComponent("BaseInfo").(*BaseInfo)
}

func (this *Player) GetBag() *Bag {
	return this.GetComponent("Bag").(*Bag)
}

func (this *Player) GetQuest() *Quest {
	return this.GetComponent("Quest").(*Quest)
}

func (this *Player) GetGuild() *Guild {
	return this.GetComponent("Guild").(*Guild)
}

// 开启消息处理协程
// 每个玩家一个独立的消息处理协程
// 除了登录消息,其他消息都在玩家自己的协程里处理,因此这里对本玩家的操作不需要加锁
func (this *Player) RunRoutine() bool {
	logger.Debug("player RunRoutine %v", this.GetId())
	return this.RunProcessRoutine(this, &gentity.RoutineEntityRoutineArgs{
		EndFunc: func(routineEntity gentity.RoutineEntity) {
			// 协程结束的时候,移除玩家
			GetPlayerMgr().RemovePlayer(this)
		},
		ProcessMessageFunc: func(routineEntity gentity.RoutineEntity, message interface{}) {
			this.processMessage(message.(*ProtoPacket))
		},
		AfterTimerExecuteFunc: func(routineEntity gentity.RoutineEntity, t time.Time) {
			// 如果有需要保存的数据修改了,即时保存数据库
			this.SaveCache(cache.Get())
		},
	})
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
	if gentity.ProcessComponentHandler(this, message.Command(), message.Message()) {
		// 如果有需要保存的数据修改了,即时保存缓存
		this.SaveCache(cache.Get())
		return
	}
	// 再找普通接口
	packetHandler := _clientConnectionHandler.GetPacketHandler(message.Command())
	if packetHandler != nil {
		packetHandler(this.GetConnection(), message)
		// 如果有需要保存的数据修改了,即时保存缓存
		this.SaveCache(cache.Get())
		return
	}
	logger.Error("unhandle message:%v", message.Command())
}

// 放入消息队列
func (this *Player) OnRecvPacket(packet *ProtoPacket) {
	logger.Debug("OnRecvPacket %v", proto.MessageName(packet.Message()).Name())
	this.PushMessage(packet)
}

// 从加载的数据构造出玩家对象
func CreatePlayerFromData(playerData *pb.PlayerData) *Player {
	player := &Player{
		name:         playerData.Name,
		accountId:    playerData.AccountId,
		regionId:     playerData.RegionId,
		BaseRoutineEntity: *gentity.NewRoutineEntity(8),
	}
	player.Id = playerData.XId
	// 初始化玩家的各个模块
	player.AddComponent(NewBaseInfo(player, playerData.BaseInfo), nil)
	player.AddComponent(NewMoney(player), playerData.Money)
	player.AddComponent(NewBag(player), playerData.Bag)
	player.AddComponent(NewQuest(player), playerData.Quest)
	player.AddComponent(NewGuild(player), playerData.Guild)
	player.AddComponent(NewPendingMessages(player), playerData.PendingMessages)
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