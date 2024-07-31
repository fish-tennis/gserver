package social

import (
	"fmt"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/game"
	"github.com/fish-tennis/gserver/internal"
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
	gentity.GetEntitySaveableStruct(tmpGuild)
	AutoRegisterGuildPacketHandler(tmpGuild, internal.HandlerMethodNamePrefix, internal.ProtoPackageName)
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
	// 如: guild.JoinRequests -> JoinRequests
	componentStructName := typ.String()[strings.LastIndex(typ.String(), ".")+1:]
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		if method.Type.NumIn() != 3 {
			continue
		}
		// 函数名前缀检查
		if !strings.HasPrefix(method.Name, methodNamePrefix) {
			continue
		}
		// 消息回调格式: func (this *GuildJoinRequests) HandleGuildJoinReq(guildMessage *GuildMessage, req *pb.GuildJoinReq)
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
			GetLogger().Debug("methodName not match:%v", method.Name)
			continue
		}
		messageId := util.GetMessageIdByMessageName(protoPackageName, componentStructName, messageName)
		if messageId == 0 {
			GetLogger().Debug("methodName match:%v but messageId==0", method.Name)
			continue
		}
		cmd := PacketCommand(messageId)
		// 注册消息回调到组件上
		_guildPacketHandlerMgr[cmd] = &internal.PacketHandlerInfo{
			ComponentName: componentName,
			Cmd:           cmd,
			Method:        method,
		}
		GetLogger().Info("GuildPacketHandler %v.%v %v", componentStructName, method.Name, messageId)
	}
}

func GuildServerHandlerRegister(handler PacketHandlerRegister) {
	slog.Info("GuildServerHandlerRegister")
	// 其他服务器转发过来的公会消息
	handler.Register(PacketCommand(pb.CmdRoute_Cmd_GuildRoutePlayerMessageReq), func(connection Connection, packet Packet) {
		req := packet.Message().(*pb.GuildRoutePlayerMessageReq)
		slog.Debug("GuildRoutePlayerMessageReq", "packet", req)
		err := ParseRoutePacket(_guildMgr, connection, packet, req.FromGuildId)
		if err != nil {
			// 回复一个结果,避免rpc调用方超时
			routePacket := NewProtoPacketEx(packet.Command()+1, nil)
			if rpcCallIdSetter, ok := packet.(RpcCallIdSetter); ok {
				routePacket.SetRpcCallId(rpcCallIdSetter.RpcCallId())
			}
			game.RoutePlayerPacketWithErr(req.FromPlayerId, routePacket, err.Error(), game.WithConnection(connection))
		}
	}, new(pb.GuildRoutePlayerMessageReq))
}

func ParseRoutePacket(mgr *gentity.DistributedEntityMgr, connection Connection, packet Packet, toEntityId int64) error {
	// 再验证一次是否属于本服务器管理
	if _guildHelper.RouteServerId(toEntityId) != gentity.GetApplication().GetId() {
		GetLogger().Warn("route err entityId:%v %v", toEntityId, packet)
		return gentity.ErrRouteServerId
	}
	toEntity := mgr.GetEntity(toEntityId)
	if toEntity == nil {
		//if mgr.loadEntityWhenGetNil {
		//	toEntity = _guildHelper.LoadEntity(toEntityId)
		//}
		toEntity = _guildHelper.LoadEntity(toEntityId)
		if toEntity == nil {
			GetLogger().Debug("ParseRoutePacket entity==nil entityId:%v %v", toEntityId, packet)
			return gentity.ErrEntityNotExists
		}
	}
	message := _guildHelper.RoutePacketToRoutineMessage(connection, packet, toEntityId)
	if message == nil {
		GetLogger().Debug("ParseRoutePacket convert err entityId:%v %v", toEntityId, packet)
		return gentity.ErrConvertRoutineMessage
	}
	toEntity.PushMessage(message)
	return nil
}
