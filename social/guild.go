package social

import (
	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/game"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"reflect"
)

var _ gentity.RoutineEntity = (*Guild)(nil)

// 公会
type Guild struct {
	gentity.BaseRoutineEntity
}

// requestPacket->route to guild->convert packet to guildMessage-->guild.PushMessage
type GuildMessage struct {
	fromPlayerId   int64
	fromServerId   int32
	fromPlayerName string
	cmd            PacketCommand
	message        proto.Message
	srcPacket      Packet     // 来源packet
	srcConnection  Connection // 来源连接,返回消息"原路返回",才能实现rpc功能
}

func NewGuild(guildLoadData *pb.GuildLoadData) *Guild {
	guild := &Guild{
		BaseRoutineEntity: *gentity.NewRoutineEntity(32),
	}
	guild.Id = guildLoadData.Id
	_guildComponentRegister.InitComponents(guild, guildLoadData)
	return guild
}

func (this *Guild) processMessage(guildMessage *GuildMessage) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("recover:%v", err)
			logger.LogStack()
		}
	}()
	// 调用注册的组件回调接口
	handlerInfo := _guildPacketHandlerMgr[guildMessage.cmd]
	if handlerInfo != nil {
		component := this.GetComponentByName(handlerInfo.ComponentName)
		if component != nil {
			// 反射调用函数
			slog.Debug("processMessage", "cmd", guildMessage.cmd, "message", proto.MessageName(guildMessage.message))
			handlerInfo.Method.Func.Call([]reflect.Value{reflect.ValueOf(component),
				reflect.ValueOf(guildMessage),
				reflect.ValueOf(guildMessage.message)})
			return
		}
	}
	slog.Warn("unhandled", "cmd", guildMessage.cmd, "message", proto.MessageName(guildMessage.message))
}

func (this *Guild) GetMember(playerId int64) *pb.GuildMemberData {
	return this.GetMembers().Get(playerId)
}

// 路由玩家消息
// this server -> other server -> player
func (this *Guild) RoutePlayerPacket(guildMessage *GuildMessage, cmd any, message proto.Message, opts ...game.RouteOption) {
	routePacket := NewProtoPacketEx(cmd, message)
	if protoPacket, ok := guildMessage.srcPacket.(*ProtoPacket); ok {
		routePacket.SetRpcCallId(protoPacket.RpcCallId())
	}
	newOpts := make([]game.RouteOption, len(opts)+1)
	// 回消息时,使用来源连接,才能让rpc调用方收到结果
	newOpts[0] = game.WithConnection(guildMessage.srcConnection)
	for i, opt := range opts {
		newOpts[i+1] = opt
	}
	game.RoutePlayerPacket(guildMessage.fromPlayerId, routePacket, newOpts...)
}

// 路由玩家消息,直接发给客户端
// this server -> other server -> client
func (this *Guild) RouteClientPacket(guildMessage *GuildMessage, cmd any, message proto.Message) {
	game.RoutePlayerPacket(guildMessage.fromPlayerId, NewProtoPacketEx(cmd, message),
		game.WithDirectSendClient(), game.WithConnection(guildMessage.srcConnection))
}

// 广播公会消息
// this server -> other server -> player
func (this *Guild) BroadcastPlayerPacket(cmd any, message proto.Message) {
	for _, member := range this.GetMembers().Data {
		game.RoutePlayerPacket(member.Id, NewProtoPacketEx(cmd, message))
	}
}

// 广播公会消息,直接发给客户端
// this server -> other server -> client
func (this *Guild) BroadcastClientPacket(cmd any, message proto.Message) {
	for _, member := range this.GetMembers().Data {
		game.RoutePlayerPacket(member.Id, NewProtoPacketEx(cmd, message), game.WithDirectSendClient())
	}
}
