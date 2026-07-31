package game

import (
	"fmt"
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
	// 断线后保留玩家的等待时间(秒),在此期间玩家可重连
	ReconnectWaitSeconds = 60
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
	// connection 的读写都在玩家协程内串行执行(网络事件通过 PushMessage 投递),无需加锁
	connection Connection
	// 断线保留期定时器是否处于激活状态
	reconnectWaitTimerActive bool
	// 事件分发的嵌套检测
	fireEventLoopChecker map[reflect.Type]int32
	postEvents           []any
	// 进度事件映射
	progressEventMapping *ProgressEventMapping
	Log                  *slog.Logger // slog.With("pid", p.GetId())
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
	if !p.useGate {
		if p.connection != nil {
			p.connection.Close()
		}
	}
	p.connection = nil
}

func (p *Player) GetConnection() Connection {
	return p.connection
}

// playerDisconnectMessage 断线内部消息,由网络协程投递,在玩家协程内消费
// 这样保证对 p.connection 的访问都在玩家自己的协程内串行执行,避免并发问题
type playerDisconnectMessage struct {
	connection Connection
}

// playerKickMessage 踢人内部消息,由网络协程投递,在玩家协程内消费
type playerKickMessage struct{}

// Kick 踢玩家下线,由网络协程调用,投递到玩家协程内执行实际的 ResetConnection + Stop
func (p *Player) Kick() {
	p.PushMessage(&playerKickMessage{})
}

// playerDirectSendMessage 直接转发给客户端的消息,由网络协程投递,在玩家协程内消费
// 避免 DirectSendClient 路径在网络协程中直接读 p.connection,与玩家协程的 ResetConnection 竞争
type playerDirectSendMessage struct {
	cmd     PacketCommand
	message proto.Message
}

// playerCheckConnectionMessage 验证连接归属并投递消息,由网络协程投递,在玩家协程内消费
// 避免在网络协程中直接读 p.connection 与玩家协程的 ResetConnection/SetConnection 竞争
type playerCheckConnectionMessage struct {
	connection Connection
	packet     *ProtoPacket
}

// DirectSendClient 由网络协程调用,投递到玩家协程内执行 SendWithCommand
func (p *Player) DirectSendClient(cmd PacketCommand, message proto.Message) {
	p.PushMessage(&playerDirectSendMessage{cmd: cmd, message: message})
}

// CheckConnectionAndRecvPacket 由网络协程调用,投递到玩家协程内验证连接归属后处理消息
func (p *Player) CheckConnectionAndRecvPacket(connection Connection, packet *ProtoPacket) {
	p.PushMessage(&playerCheckConnectionMessage{connection: connection, packet: packet})
}

// playerReconnectMessage 重连内部消息,由网络协程投递,在玩家协程内消费
// 保证对 BaseInfo.ReconnectSession 的校验、SetConnection、CancelReconnectWait 都在协程内串行执行
type playerReconnectMessage struct {
	connection Connection
	useGate    bool
	session    string
	callback   func(success bool)
}

// OnDisconnect 由网络协程(gnet读/写协程)回调,不能在此直接读写 p.connection
// 投递到玩家自己的协程中处理
func (p *Player) OnDisconnect(connection Connection) {
	p.PushMessage(&playerDisconnectMessage{connection: connection})
}

// OnReconnect 由网络协程(收包协程)调用,把重连校验逻辑投递到玩家协程
// callback 在玩家协程内被调用,success 为 true 表示校验通过且已绑定新连接
func (p *Player) OnReconnect(connection Connection, useGate bool, session string, callback func(success bool)) {
	p.PushMessage(&playerReconnectMessage{
		connection: connection,
		useGate:    useGate,
		session:    session,
		callback:   callback,
	})
}

// onDisconnect 实际的断线处理逻辑,在玩家协程内调用,与本玩家其它业务串行,无需加锁
func (p *Player) onDisconnect(connection Connection) {
	if p.GetConnection() == connection {
		p.ResetConnection()
		// 重复掉线场景:保留期定时器已经处于激活状态,直接返回
		if p.reconnectWaitTimerActive {
			return
		}
		// 开启断线保留期,等待玩家重连,超时后才真正下线
		p.reconnectWaitTimerActive = true
		p.GetTimerEntries().After(ReconnectWaitSeconds*time.Second, func() time.Duration {
			// 保留期内重连成功会取消该定时器(reconnectWaitTimerActive被置为false)
			if !p.reconnectWaitTimerActive {
				return 0
			}
			p.Stop()
			logger.Debug("player %v exit", p.GetId())
			return 0
		})
	}
}

// CancelReconnectWait 取消断线保留期定时器,由重连处理器在玩家协程内调用
func (p *Player) CancelReconnectWait() {
	p.reconnectWaitTimerActive = false
}

// onReconnect 实际的重连处理逻辑,在玩家协程内调用,与本玩家其它业务串行,无需加锁
func (p *Player) onReconnect(msg *playerReconnectMessage) {
	// 校验 ReconnectSession
	if !p.GetBaseInfo().VerifyReconnectSession(msg.session) {
		msg.callback(false)
		return
	}
	// 校验通过,绑定新连接
	p.SetConnection(msg.connection, msg.useGate)
	// 取消断线保留期定时器
	p.CancelReconnectWait()
	msg.callback(true)
}

// 发包(protobuf)
// NOTE:调用Send(message)之后,不要再对message进行读写!
func (p *Player) Send(message proto.Message, opts ...SendOption) bool {
	cmd := network.GetCommandByProto(message)
	if cmd <= 0 {
		slog.Error("CmdNotFound", "pid", p.GetId(), "messageName", proto.MessageName(message))
		return false
	}
	return p.SendWithCommand(PacketCommand(cmd), message, opts...)
}

func (p *Player) SendWithCommand(cmd PacketCommand, message proto.Message, opts ...SendOption) bool {
	if p.connection != nil {
		// 在业务协程中序列化proto,防止message在gnet的网络协程中序列化时,业务层如果继续修改该message可能有并发问题
		// 尤其是message有引用性的字段时
		bytes, err := proto.Marshal(message)
		if err != nil {
			p.Log.Error("SendWithCommandErr", "msg", proto.MessageName(message).Name(), "err", err)
			return false
		}
		if p.useGate {
			// 网关模式,自动附加上playerId
			return p.connection.SendPacket(network.NewGatePacketWithData(p.GetId(), cmd, bytes), opts...)
		} else {
			return p.connection.SendPacket(NewProtoPacketWithData(cmd, bytes), opts...)
		}
	}
	return false
}

func (p *Player) SendPacket(packet Packet, opts ...SendOption) bool {
	if p.connection != nil {
		if p.useGate {
			bytes := packet.GetStreamData()
			if len(bytes) == 0 && packet.Message() != nil {
				var err error
				bytes, err = proto.Marshal(packet.Message())
				if err != nil {
					p.Log.Error("SendPacketErr", "msg", proto.MessageName(packet.Message()).Name(), "err", err)
					return false
				}
			}
			// 网关模式,自动附加上playerId
			return p.connection.SendPacket(network.NewGatePacketWithData(p.GetId(), packet.Command(), bytes).
				WithRpc(packet), opts...)
		} else {
			return p.connection.SendPacket(packet, opts...)
		}
	}
	return false
}

// 通用的错误返回消息
func (p *Player) SendErrorRes(errorReqCmd PacketCommand, errorMsg string, opts ...SendOption) bool {
	return p.SendWithCommand(network.ErrorResCmd, &pb.ErrorRes{
		Command:   int32(errorReqCmd),
		ResultStr: errorMsg,
	}, opts...)
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
	logger.Debug("%v PostEvent:%v", p.GetId(), event)
	// 先保存起来,再延后执行
	p.postEvents = append(p.postEvents, event)
}

func (p *Player) firePostedEvents() {
	for i := 0; i < int(internal.SameEventLoopLimit); i++ {
		if len(p.postEvents) == 0 {
			break
		}
		postEvents := p.postEvents
		p.postEvents = nil
		for _, event := range postEvents {
			p.FireEvent(event) // 执行过程中有可能又触发了p.PostEvent
		}
	}
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
			p.firePostedEvents()
			// 协程结束的时候,移除玩家
			GetPlayerMgr().RemovePlayer(p)
		},
		ProcessMessageFunc: func(routineEntity gentity.RoutineEntity, message any) {
			switch msg := message.(type) {
			case *ProtoPacket:
				p.processMessage(msg)
			case *playerDisconnectMessage:
				p.onDisconnect(msg.connection)
			case *playerReconnectMessage:
				p.onReconnect(msg)
			case *playerKickMessage:
				p.ResetConnection()
				p.Stop()
			case *playerDirectSendMessage:
				p.SendWithCommand(msg.cmd, msg.message)
			case *playerCheckConnectionMessage:
				if p.GetConnection() == msg.connection {
					p.processMessage(msg.packet)
				}
			default:
				logger.Error("processMessage unknown message type: %T", message)
			}
		},
		AfterTimerExecuteFunc: func(routineEntity gentity.RoutineEntity, t time.Time) {
			p.firePostedEvents()
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
				Current:  p.GetPropertyInt32("OnlineMinute", nil),
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
	p.Log.Debug("processMessage", "msg", proto.MessageName(message.Message()).Name())
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
		// NOTE:这里加了超时时间,防止Send阻塞导致玩家协程阻塞
		if resErr != nil {
			p.SendErrorRes(handlerInfo.ResCmd, resErr.Error(), Timeout(time.Second))
		} else {
			p.Send(resProto, Timeout(time.Second))
		}
	}) {
		p.firePostedEvents()
		// 如果有需要保存的数据修改了,即时保存缓存
		p.SaveCache(cache.Get())
		return
	}
	p.Log.Error("processMessageUnhandled", "msg", proto.MessageName(message.Message()).Name())
}

// 放入消息队列
func (p *Player) OnRecvPacket(packet *ProtoPacket) {
	p.Log.Debug("OnRecvPacket", "msg", proto.MessageName(packet.Message()).Name())
	p.PushMessage(packet)
}

// 玩家进入游戏服
func (p *Player) HandlePlayerEntryGameOk(msg *pb.PlayerEntryGameOk) {
	p.Log.Debug("HandlePlayerEntryGameOk", "msg", msg)
	// 同步各模块的数据给客户端
	p.RangeComponent(func(component gentity.Component) bool {
		if dataSyncer, ok := component.(DataSyncer); ok {
			dataSyncer.SyncDataToClient()
			p.Log.Debug("SyncDataToClient", "component", component.GetName())
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
	// 进游戏服成功后生成 ReconnectSession,供后续断线重连校验
	p.GetBaseInfo().GenerateReconnectSession()
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
		Log:               slog.With("pid", playerId),
	}
	player.Id = playerId
	player.progressEventMapping = &ProgressEventMapping{
		player: player,
	}
	// 初始化玩家的各个模块
	_playerComponentRegister.InitComponents(player, nil)
	player.Log.Debug("CreatePlayer")
	return player
}

// 从加载的数据构造出玩家对象
func CreatePlayerFromData(playerData *pb.PlayerData) *Player {
	var player *Player
	defer func() {
		if err := recover(); err != nil {
			player = nil
			slog.Error("CreatePlayerFromDataErr", "pid", playerData.XId, "err", fmt.Sprintf("%v", err))
			LogStack()
		}
	}()
	player = CreatePlayer(playerData.XId, playerData.Name, playerData.AccountId, playerData.RegionId)
	err := gentity.LoadEntityData(player, playerData)
	if err != nil {
		player.Log.Error("LoadPlayerDataErr", "err", err)
		return nil
	}
	player.RangeComponent(func(component gentity.Component) bool {
		if dataLoader, ok := component.(internal.DataLoader); ok {
			dataLoader.OnDataLoad()
			player.Log.Debug("OnDataLoad", "component", component.GetName())
		}
		return true
	})
	return player
}

func CreateTempPlayer(playerId, accountId int64) *Player {
	return CreatePlayer(playerId, "", accountId, 0)
}

// 获取金币数量(金币就是背包里的一件普通物品)
func (p *Player) GetCoin() int32 {
	return p.GetBags().GetItemCount(int32(pb.ItemId_ItemId_Coin))
}
