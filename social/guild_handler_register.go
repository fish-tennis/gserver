package social

import (
	"fmt"
	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/game"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"reflect"
	"strings"
)

// 公会消息回调接口注册
var _guildPacketHandlerMgr = make(map[PacketCommand]*internal.PacketHandlerInfo)

func InitGuildStructAndHandler() {
	tmpGuild := NewGuild(&pb.GuildLoadData{})
	gentity.ParseEntitySaveableStruct(tmpGuild)
	AutoRegisterGuildPacketHandler(tmpGuild, internal.HandlerMethodNamePrefix, network.ProtoPackageName)
}

// 公会的消息回调接口使用了特殊的形式,所以自己写接口注册流程
func AutoRegisterGuildPacketHandler(entity gentity.Entity, methodNamePrefix, protoPackageName string) {
	scanGuildMethods(entity, methodNamePrefix, protoPackageName)
	entity.RangeComponent(func(component gentity.Component) bool {
		scanGuildMethods(component, methodNamePrefix, protoPackageName)
		return true
	})
}

func scanGuildMethods(obj any, methodNamePrefix, protoPackageName string) {
	typ := reflect.TypeOf(obj)
	componentName := ""
	component, ok := obj.(gentity.Component)
	if ok {
		componentName = component.GetName()
	}
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		if method.Type.NumIn() != 3 {
			continue
		}
		// 函数名前缀检查
		if !strings.HasPrefix(method.Name, methodNamePrefix) {
			continue
		}
		// rpc回调格式:func (c *Component) HandleXxxReq(m *GuildMessage, req *pb.XxxReq) (*pb.XxxRes,error)
		methodArg1 := method.Type.In(1)
		if !methodArg1.ConvertibleTo(reflect.TypeOf(&GuildMessage{})) {
			continue
		}
		methodArg2 := method.Type.In(2)
		// 参数2是proto定义的消息体
		if !methodArg2.Implements(reflect.TypeOf((*proto.Message)(nil)).Elem()) {
			continue
		}
		// 消息名,如: GuildJoinReq
		// *pb.GuildJoinReq -> GuildJoinReq
		messageName := methodArg2.String()[strings.LastIndex(methodArg2.String(), ".")+1:]
		// 函数名规则,如HandleGuildJoinReq
		if method.Name != fmt.Sprintf("%v%v", methodNamePrefix, messageName) {
			slog.Debug("methodName not match", "method", method.Name)
			continue
		}
		reqCmd := network.GetCommandByProto(reflect.New(methodArg2.Elem()).Interface().(proto.Message))
		if reqCmd == 0 {
			slog.Debug("reqCmd==0", "method", method.Name)
			continue
		}
		var (
			resCmd         int32
			resMessageElem reflect.Type
		)
		if method.Type.NumOut() < 2 {
			slog.Debug("len(returnValues)<2", "method", method.Name)
			continue
		}
		resArg := method.Type.Out(0)
		// 返回值1是proto定义的消息体
		if !resArg.Implements(reflect.TypeOf((*proto.Message)(nil)).Elem()) {
			slog.Debug("resArg not proto.Message", "method", method.Name)
			continue
		}
		resMessageElem = resArg.Elem()
		resCmd = network.GetCommandByProto(reflect.New(resMessageElem).Interface().(proto.Message))
		if resCmd == 0 {
			slog.Debug("resCmd==0", "method", method.Name)
			continue
		}
		// 注册消息回调到组件上
		_guildPacketHandlerMgr[PacketCommand(reqCmd)] = &internal.PacketHandlerInfo{
			ComponentName:  componentName,
			Cmd:            PacketCommand(reqCmd),
			ResCmd:         PacketCommand(resCmd),
			ResMessageElem: resMessageElem,
			Method:         method,
		}
		slog.Info("scanGuildMethods", "component", componentName, "method", method.Name, "cmd", reqCmd)
	}
}

func GuildServerHandlerRegister(handler PacketHandlerRegister) {
	slog.Info("GuildServerHandlerRegister")
	// 其他服务器转发过来的公会消息
	handler.Register(PacketCommand(pb.CmdServer_Cmd_GuildRoutePlayerMessageReq), func(connection Connection, packet Packet) {
		req := packet.Message().(*pb.GuildRoutePlayerMessageReq)
		slog.Debug("GuildRoutePlayerMessageReq", "packet", req)
		err := ParseRoutePacket(_guildMgr, connection, packet, req.FromGuildId)
		if err != nil {
			// 回复一个结果,避免rpc调用方超时
			routePacket := NewProtoPacketEx(network.GetResCommand(req.PacketCommand), nil)
			routePacket.SetRpcCallId(packet.RpcCallId())
			game.RoutePlayerPacket(req.FromPlayerId, routePacket, game.WithConnection(connection), game.WithError(err))
		}
	}, new(pb.GuildRoutePlayerMessageReq))
}

func ParseRoutePacket(mgr *gentity.DistributedEntityMgr, connection Connection, packet Packet, toEntityId int64) error {
	log := slog.Default().With("toEntityId", toEntityId, "packet", packet)
	// 再验证一次是否属于本服务器管理
	if _guildHelper.RouteServerId(toEntityId) != gentity.GetApplication().GetId() {
		log.Warn("route err")
		return gentity.ErrRouteServerId
	}
	toEntity := mgr.GetEntity(toEntityId)
	if toEntity == nil {
		//if mgr.loadEntityWhenGetNil {
		//	toEntity = _guildHelper.LoadEntity(toEntityId)
		//}
		toEntity = _guildHelper.LoadEntity(toEntityId)
		if toEntity == nil {
			log.Debug("ParseRoutePacket entity==nil")
			return gentity.ErrEntityNotExists
		}
	}
	message := _guildHelper.RoutePacketToRoutineMessage(connection, packet, toEntityId)
	if message == nil {
		log.Debug("ParseRoutePacket convert err")
		return gentity.ErrConvertRoutineMessage
	}
	toEntity.PushMessage(message)
	return nil
}
