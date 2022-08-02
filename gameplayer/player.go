package gameplayer

import (
	"context"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/db"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"reflect"
	"sync"
)

var _ Entity = (*Player)(nil)

// 玩家对象
type Player struct {
	BaseEntity
	// 玩家唯一id
	id int64
	// 玩家名(unique)
	name string
	// 账号id
	accountId int64
	// 区服id
	regionId int32
	//accountName string
	// 关联的连接
	connection Connection
	// 消息队列
	messages chan *ProtoPacket
	stopChan chan struct{}
	stopOnce sync.Once
	// 倒计时管理
	timerEntries *TimerEntries
}

// 玩家唯一id
func (this *Player) GetId() int64 {
	return this.id
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
func (this *Player) GetComponent(componentName string) Component {
	index := GetComponentIndex(componentName)
	if index >= 0 {
		return this.GetComponentByIndex(index)
	}
	return nil
}

// 玩家数据保存数据库
func (this *Player) SaveDb(removeCacheAfterSaveDb bool) error {
	return SaveEntityToDb_New(db.GetPlayerDb(), this, removeCacheAfterSaveDb)
	//return SaveEntityToDb(db.GetPlayerDb(), this, removeCacheAfterSaveDb)
}

// 设置关联的连接
func (this *Player) SetConnection(connection Connection) {
	// 取消之前的连接和该玩家的关联
	if this.connection != nil && this.connection != connection {
		this.connection.SetTag(nil)
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
		return this.connection.Send(command, message)
	}
	return false
}

// 分发事件给组件
func (this *Player) FireEvent(event interface{}) {
	logger.Debug("%v FireEvent:%v", this.GetId(), event)
	this.RangeComponent(func(component Component) bool {
		if eventReceiver, ok := component.(EventReceiver); ok {
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

func (this *Player) GetQuest() *Quest {
	return this.GetComponent("Quest").(*Quest)
}

func (this *Player) GetGuild() *Guild {
	return this.GetComponent("Guild").(*Guild)
}

func (this *Player) GetTimerEntries() *TimerEntries {
	return this.timerEntries
}

// 开启消息处理协程
// 每个玩家一个独立的消息处理协程
// 除了登录消息,其他消息都在玩家自己的协程里处理,因此这里对本玩家的操作不需要加锁
func (this *Player) StartProcessRoutine() bool {
	GetServer().GetWaitGroup().Add(1)
	go func(ctx context.Context) {
		defer func() {
			// 协程结束的时候,移除玩家
			GetPlayerMgr().RemovePlayer(this)
			GetServer().GetWaitGroup().Done()
			if err := recover(); err != nil {
				logger.LogStack()
			}
			logger.Debug("EndProcessRoutine %v", this.GetId())
		}()

		this.timerEntries.Start()
		for {
			select {
			case <-ctx.Done():
				logger.Info("exitNotify")
				goto END
			case <-this.stopChan:
				logger.Debug("stop")
				goto END
			case message := <-this.messages:
				// nil消息 表示这是需要处理的最后一条消息
				if message == nil {
					return
				}
				this.processMessage(message)
			case timeNow := <-this.timerEntries.TimerChan():
				// 计时器的回调在玩家协程里执行,所以是协程安全的
				this.timerEntries.Run(timeNow)
			}
		}

		// 有可能还有未处理的消息
	END:
		messageLen := len(this.messages)
		for i := 0; i < messageLen; i++ {
			message := <-this.messages
			// nil消息 表示这是需要处理的最后一条消息
			if message == nil {
				return
			}
			this.processMessage(message)
		}
	}(GetServer().GetContext())
	return true
}

func (this *Player) processMessage(message *ProtoPacket) {
	defer func() {
		if err := recover(); err != nil {
			logger.LogStack()
		}
	}()
	logger.Debug("processMessage %v", proto.MessageName(message.Message()).Name())
	// 先找组件接口
	handlerInfo := _playerComponentHandlerInfos[message.Command()]
	if handlerInfo != nil {
		// 在线玩家的消息,自动路由到对应的玩家组件上
		component := this.GetComponent(handlerInfo.componentName)
		if component != nil {
			if handlerInfo.handler != nil {
				handlerInfo.handler(component, message.Message())
			} else {
				// 用了反射,性能有所损失
				handlerInfo.method.Func.Call([]reflect.Value{reflect.ValueOf(component), reflect.ValueOf(message.Message())})
			}
			// 如果有需要保存的数据修改了,即时保存数据库
			this.SaveCache()
			return
		}
	}
	// 再找普通接口
	packetHandler := _clientConnectionHandler.GetPacketHandler(message.Command())
	if packetHandler != nil {
		packetHandler(this.GetConnection(), message)
		// 如果有需要保存的数据修改了,即时保存数据库
		this.SaveCache()
		return
	}
	logger.Error("unhandle message:%v", message.Command())
}

// 收到网络消息,先放入消息队列
func (this *Player) OnRecvPacket(packet *ProtoPacket) {
	this.messages <- packet
	logger.Debug("OnRecvPacket %v", proto.MessageName(packet.Message()).Name())
}

// 停止协程
func (this *Player) Stop() {
	this.stopOnce.Do(func() {
		this.stopChan <- struct{}{}
	})
}

// 从加载的数据构造出玩家对象
func CreatePlayerFromData(playerData *pb.PlayerData) *Player {
	player := &Player{
		id:           playerData.Id,
		name:         playerData.Name,
		accountId:    playerData.AccountId,
		regionId:     playerData.RegionId,
		messages:     make(chan *ProtoPacket, 8),
		stopChan:     make(chan struct{}, 1),
		timerEntries: NewTimerEntries(),
	}
	// 初始化玩家的各个模块
	player.AddComponent(NewBaseInfo(player, playerData.BaseInfo), nil)
	player.AddComponent(NewMoney(player), playerData.Money)
	bagCountItem := NewBagCountItem(player, playerData.BagCountItem)
	player.AddComponent(bagCountItem, nil)
	bagUniqueItem := NewBagUniqueItem(player)
	player.AddComponent(bagUniqueItem, playerData.BagUniqueItem)
	player.AddComponent(NewBag(player, bagCountItem, bagUniqueItem), nil)
	player.AddComponent(NewQuest(player), playerData.Quest)
	player.AddComponent(NewGuild(player), playerData.Guild)
	return player
}

func CreateTempPlayer(playerId, accountId int64) *Player {
	playerData := &pb.PlayerData{}
	player := CreatePlayerFromData(playerData)
	player.id = playerId
	player.accountId = accountId
	return player
}
