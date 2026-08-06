package game

import (
	"log/slog"
	"reflect"
	"time"

	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
)

const (
	// Player在redis里的前缀
	PlayerCachePrefix = "p"
	// ReconnectWaitSeconds 玩家掉线后的保留期(秒),期间等待客户端重连,超时则正式下线
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
	// 断线保留期定时器的代际计数器,用于区分不同轮次的定时器
	// 防止陈旧定时器在新一轮保留期内误触发 Stop
	reconnectWaitGen int
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
// 仅在新玩家创建(RunRoutine之前)或玩家协程事件处理中调用,无并发竞争
func (p *Player) SetConnection(connection Connection, useGate bool) {
	p.useGate = useGate
	if !useGate {
		// 顶号场景:关闭旧连接,释放其底层资源,避免连接泄漏
		// 仅清除 tag 不够:旧连接会保持打开状态,直到客户端超时或 TCP keepalive 探测失败
		if p.connection != nil && p.connection != connection {
			p.connection.SetTag(nil)
			p.connection.Close()
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

// PlayerDirectSendMessage 是 playerDirectSendMessage 的导出版本,供跨包构造
type PlayerDirectSendMessage struct {
	Cmd     PacketCommand
	Message proto.Message
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

// CheckConnectionAndRecvClientPacket 由网络协程调用,投递到玩家协程内验证连接归属后处理消息
// 使用 TryPushMessage 非阻塞投递:玩家 channel 满时返回 false,调用方负责返回错误给客户端
// 防止某个慢玩家阻塞 gate 连接的收包 goroutine,影响所有其他客户端
func (p *Player) CheckConnectionAndRecvClientPacket(connection Connection, packet *ProtoPacket) bool {
	return p.TryPushMessage(&playerCheckConnectionMessage{connection: connection, packet: packet})
}

// playerReconnectMessage 重连内部消息,由网络协程投递,在玩家协程内消费
// 保证对 BaseInfo.ReconnectSession 的校验、SetConnection、CancelReconnectWait 都在协程内串行执行
type playerReconnectMessage struct {
	connection Connection
	useGate    bool
	session    string
	packet     Packet
}

// playerEntryReconnectMessage 通过 PlayerEntryGameReq 触发的隐式重连消息
// 由收包协程投递,在玩家协程内消费
// 与 playerReconnectMessage 的区别:无需 ReconnectSession 校验(已通过 LoginSession 验证)
// 响应发送也在协程内完成,消除 GetPlayer 与 RemovePlayer 的 check-then-act 竞态
type playerEntryReconnectMessage struct {
	connection Connection
	useGate    bool
	req        *pb.PlayerEntryGameReq
	packet     Packet
}

// OnDisconnect 由网络协程(gnet读/写协程)回调,不能在此直接读写 p.connection
// 投递到玩家自己的协程中处理
func (p *Player) OnDisconnect(connection Connection) {
	p.PushMessage(&playerDisconnectMessage{connection: connection})
}

// OnReconnect 由DB协程池调用,把重连校验逻辑投递到玩家协程
// 使用 TryPushMessage:玩家 channel 满时返回 false,调用方返回 TryLater 给客户端
func (p *Player) OnReconnect(connection Connection, useGate bool, session string, packet Packet) bool {
	return p.TryPushMessage(&playerReconnectMessage{
		connection: connection,
		useGate:    useGate,
		session:    session,
		packet:     packet,
	})
}

// OnEntryReconnect 玩家通过 PlayerEntryGameReq 隐式重连(已通过 LoginSession 验证)
// 使用 TryPushMessage:玩家 channel 满时返回 false,调用方返回 TryLater 给客户端
func (p *Player) OnEntryReconnect(connection Connection, useGate bool, req *pb.PlayerEntryGameReq, packet Packet) bool {
	return p.TryPushMessage(&playerEntryReconnectMessage{
		connection: connection,
		useGate:    useGate,
		req:        req,
		packet:     packet,
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
		p.reconnectWaitGen++
		gen := p.reconnectWaitGen
		p.GetTimerEntries().After(ReconnectWaitSeconds*time.Second, func() time.Duration {
			// 检查代际:如果重连成功后再次断线,旧定时器的 gen 不匹配,直接忽略
			if !p.reconnectWaitTimerActive || p.reconnectWaitGen != gen {
				return 0
			}
			p.Stop()
			slog.Debug("player exit", "pid", p.GetId())
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
	res := &pb.PlayerReconnectGameRes{
		AccountId: p.GetAccountId(),
		PlayerId:  p.GetId(),
	}
	// 校验 ReconnectSession
	if !p.GetBaseInfo().VerifyReconnectSession(msg.session) {
		network.SendPacketAdaptWithError(msg.connection, msg.packet, res, int32(pb.ErrorCode_ErrorCode_ReconnectSessionError))
		slog.Debug("onReconnect session error", "pid", p.GetId())
		return
	}
	// 校验通过,绑定新连接
	p.SetConnection(msg.connection, msg.useGate)
	// 取消断线保留期定时器
	p.CancelReconnectWait()
	// 发送重连成功响应
	res.GameServerId = gentity.GetApplication().GetId()
	network.SendPacketAdaptWithError(msg.connection, msg.packet, res, 0)
	// 处理重连后的逻辑(与正常登录走相同的 PlayerEntryGameOk 流程)
	// ReconnectSession 的轮换在 HandlePlayerEntryGameOk 中统一处理,避免重复调用
	cmd := network.GetCommandByProto(new(pb.PlayerEntryGameOk))
	p.processMessage(NewProtoPacket(PacketCommand(cmd), &pb.PlayerEntryGameOk{
		IsReconnect: true,
	}))
	slog.Debug("onReconnect success", "pid", p.GetId(), "res", res)
}

// onEntryReconnect 隐式重连的实际处理逻辑,在玩家协程内调用
// 与 onReconnect 的区别:跳过 ReconnectSession 校验(LoginSession 已在收包协程验证通过)
// 响应在协程内发送,消除 GetPlayer 与 RemovePlayer 的 check-then-act 竞态:
// 即使投递消息时玩家协程已停止,PushMessage 不会阻塞,消息被丢弃,但收包协程不会误发成功响应
func (p *Player) onEntryReconnect(msg *playerEntryReconnectMessage) {
	res := &pb.PlayerEntryGameRes{
		AccountId: msg.req.AccountId,
		RegionId:  msg.req.RegionId,
	}
	// 绑定新连接(与玩家协程的ResetConnection/Send串行,无竞态)
	p.SetConnection(msg.connection, msg.useGate)
	// 取消断线保留期定时器:进游成功后玩家已恢复在线
	p.CancelReconnectWait()
	// 填充响应
	res.PlayerId = p.GetId()
	res.PlayerName = p.GetName()
	res.GameServerId = int32(gentity.GetApplication().GetId())
	// 发送进游成功响应(在协程内发送,确保只在协程存活时才响应)
	network.SendPacketAdaptWithError(msg.connection, msg.packet, res, 0)
	// 投递进游消息:生成新ReconnectSession并同步各模块数据给客户端
	// IsReconnect=false:隐式重连对客户端而言是全新登录(杀进程后重启),不是断线恢复
	cmd := network.GetCommandByProto(new(pb.PlayerEntryGameOk))
	p.processMessage(NewProtoPacket(PacketCommand(cmd), &pb.PlayerEntryGameOk{
		IsReconnect: false,
	}))
	slog.Debug("onEntryReconnect success", "pid", p.GetId(), "res", res)
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

// 带 rpcCallId 发送响应(框架自动透传用,业务层无需关心)
// rpcCallId==0 时退化为不写标记位,与改动前字节完全一致
func (p *Player) SendWithCommandAndRpc(cmd PacketCommand, message proto.Message, rpcCallId uint32, opts ...SendOption) bool {
	conn := p.connection
	useGate := p.useGate
	if conn == nil {
		return false
	}
	// 在业务协程中序列化proto,防止message在gnet的网络协程中序列化时,业务层如果继续修改该message可能有并发问题
	bytes, err := proto.Marshal(message)
	if err != nil {
		p.Log.Error("SendWithCommandAndRpcErr", "msg", proto.MessageName(message).Name(), "err", err)
		return false
	}
	if useGate {
		// 网关模式:用 GatePacket 携带 rpcCallId,codec 仅在 rpcCallId>0 时写入字节
		return conn.SendPacket(network.NewGatePacketWithData(p.GetId(), cmd, bytes).WithRpc(rpcCallId), opts...)
	}
	// 直连模式:用 ProtoPacket 携带 rpcCallId
	return conn.SendPacket(NewProtoPacketWithData(cmd, bytes).WithRpc(rpcCallId), opts...)
}

// 通用的错误返回消息(带 rpcCallId)
func (p *Player) SendErrorResWithRpc(errorReqCmd PacketCommand, errorMsg string, rpcCallId uint32, opts ...SendOption) bool {
	cmd := network.GetCommandByProto(new(pb.ErrorRes))
	return p.SendWithCommandAndRpc(PacketCommand(cmd), &pb.ErrorRes{
		Command:   int32(errorReqCmd),
		ResultStr: errorMsg,
	}, rpcCallId, opts...)
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
	slog.Debug("FireConditionEvent", "playerId", p.GetId(), "event", event)
	// 进度更新
	p.progressEventMapping.OnTriggerEvent(event)
}

// 分发事件,但是延后执行
func (p *Player) PostEvent(event any) {
	slog.Debug("PostEvent", "playerId", p.GetId(), "event", event)
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
	// 超过嵌套上限仍有未处理事件,告警(可能存在事件循环死锁或链路过深)
	if len(p.postEvents) > 0 {
		slog.Error("firePostedEvents dropped: event loop limit exceeded",
			"pid", p.GetId(),
			"limit", internal.SameEventLoopLimit,
			"droppedCount", len(p.postEvents))
		p.postEvents = nil
	}
}

func (p *Player) GetLevel() int32 {
	return p.GetBaseInfo().Data.Level
}

// 开启消息处理协程
// 每个玩家一个独立的消息处理协程
// 除了登录消息,其他消息都在玩家自己的协程里处理,因此这里对本玩家的操作不需要加锁
func (p *Player) RunRoutine() bool {
	slog.Debug("player RunRoutine", "playerId", p.GetId())
	ok := p.RunProcessRoutine(p, &gentity.RoutineEntityRoutineArgs{
		EndFunc: func(routineEntity gentity.RoutineEntity) {
			// defer 确保 RemovePlayer 总是执行,即使 FireEvent/firePostedEvents panic
			// 否则 playerWg.Done() 不会被调用,Exit() 中的 playerWg.Wait() 会永久阻塞
			defer GetPlayerMgr().RemovePlayer(p)
			// 分发事件:玩家退出游戏
			p.FireEvent(&internal.EventPlayerExit{})
			p.firePostedEvents()
		},
		ProcessMessageFunc: func(routineEntity gentity.RoutineEntity, message any) {
			switch msg := message.(type) {
			case *ProtoPacket:
				p.processMessage(msg)
			case *playerDisconnectMessage:
				p.onDisconnect(msg.connection)
			case *playerReconnectMessage:
				p.onReconnect(msg)
			case *playerEntryReconnectMessage:
				// 隐式重连:已通过LoginSession验证,直接绑定新连接
				p.onEntryReconnect(msg)
			case *playerKickMessage:
				p.ResetConnection()
				p.Stop()
			case *playerDirectSendMessage:
				p.SendWithCommand(msg.cmd, msg.message)
			case *PlayerDirectSendMessage:
				p.SendWithCommand(msg.Cmd, msg.Message)
			case *playerCheckConnectionMessage:
				if p.GetConnection() == msg.connection {
					p.processMessage(msg.packet)
				}
			default:
				slog.Error("ProcessMessageFunc: unknown message type", "type", reflect.TypeOf(message))
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
			slog.Error("recover", "error", err)
			LogStack()
			internal.SendAlert(err)
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
		// 框架代发的标准 Req/Res 响应自动透传请求包的 rpcCallId,业务 handler 无需感知
		// rpcCallId==0 时走原逻辑,保证与改动前字节完全一致
		rpcCallId := message.RpcCallId()
		if rpcCallId > 0 {
			if resErr != nil {
				p.SendErrorResWithRpc(handlerInfo.ResCmd, resErr.Error(), rpcCallId, Timeout(time.Second))
			} else {
				p.SendWithCommandAndRpc(handlerInfo.ResCmd, resProto, rpcCallId, Timeout(time.Second))
			}
		} else {
			if resErr != nil {
				p.SendErrorRes(handlerInfo.ResCmd, resErr.Error(), Timeout(time.Second))
				p.Log.Error("processMessageError", "msg", proto.MessageName(message.Message()).Name(), "err", resErr.Error())
			} else {
				p.Send(resProto, Timeout(time.Second))
			}
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
	// 进游时(无论是首次进游还是重连)生成新的重连校验session,供下次重连验证
	// 必须在SyncDataToClient之前生成,这样BaseInfoSync能把最新的ReconnectSession同步给客户端
	p.GetBaseInfo().GenerateReconnectSession()
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
		BaseRoutineEntity: *gentity.NewRoutineEntity(512),
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
			slog.Error("CreatePlayerFromDataErr", "pid", playerData.XId, "err", err)
			LogStack()
			internal.SendAlert(err)
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
