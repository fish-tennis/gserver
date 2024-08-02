package game

import (
	"github.com/fish-tennis/gentity/util"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"time"
)

type RouteOption interface {
	apply(*routeOptions)
}

// 路由消息的参数
type routeOptions struct {
	// true:消息直接发给客户端
	// false:放入玩家消息队列,消息将在玩家协程中被处理
	DirectSendClient bool

	// 指定连接
	Connection Connection

	// 路由到指定的服务器
	ToServerId int32

	// 先保存到数据库(player.pendingmessages),防止路由失败造成消息丢失
	SaveDb bool
}

func defaultRouteOptions() *routeOptions {
	return &routeOptions{}
}

// funcRouteOption wraps a function that modifies routeOptions into an
// implementation of the RouteOption interface.
type funcRouteOption struct {
	f func(*routeOptions)
}

func (fro *funcRouteOption) apply(ro *routeOptions) {
	fro.f(ro)
}

func newFuncRouteOption(f func(*routeOptions)) *funcRouteOption {
	return &funcRouteOption{
		f: f,
	}
}

func WithDirectSendClient() RouteOption {
	return newFuncRouteOption(func(options *routeOptions) {
		options.DirectSendClient = true
	})
}

func WithSaveDb() RouteOption {
	return newFuncRouteOption(func(options *routeOptions) {
		options.SaveDb = true
	})
}

func WithToServerId(toServerId int32) RouteOption {
	return newFuncRouteOption(func(options *routeOptions) {
		options.ToServerId = toServerId
	})
}

func WithConnection(connection Connection) RouteOption {
	return newFuncRouteOption(func(options *routeOptions) {
		options.Connection = connection
	})
}

// 路由玩家消息
// ServerA -> ServerB -> Player
//
// WithDirectSendClient():
// 消息直接转发给客户端,不做逻辑处理 ServerA -> ServerB -> Client
//
// 举例:
// 有人申请加入公会,公会广播该消息给公会成员,ServerB收到消息后,直接把消息发给客户端(Player.Send),而不需要放入玩家的逻辑消息队列(Player.OnRecvPacket)
//
// WithSaveDb(): 消息先保存数据库再转发,防止丢失
// 举例:
// 公会会长同意了玩家A的入会申请,此时玩家A可能不在线,就把该消息存入玩家的数据库,待玩家下次上线时,从数据库取出该消息,并进行相应的逻辑处理
func RoutePlayerPacket(playerId int64, packet Packet, opts ...RouteOption) bool {
	return RoutePlayerPacketWithErr(playerId, packet, "", opts...)
}

func RoutePlayerPacketWithErr(playerId int64, packet Packet, errStr string, opts ...RouteOption) bool {
	routeOpts := defaultRouteOptions()
	for _, opt := range opts {
		opt.apply(routeOpts)
	}
	pendingMessageId := int64(0)
	if routeOpts.SaveDb {
		anyMessage, err := anypb.New(packet.Message())
		if err != nil {
			logger.Error("RoutePlayerPacket %v err:%v", playerId, err)
			return false
		}
		pendingMessageId = util.GenUniqueId()
		pendingMessage := &pb.PendingMessage{
			MessageId:     pendingMessageId, // 消息号生成唯一id
			PacketCommand: int32(packet.Command()),
			PacketData:    anyMessage,
			Timestamp:     time.Now().Unix(),
		}
		pendingMessageBytes, err := proto.Marshal(pendingMessage)
		if err != nil {
			logger.Error("RoutePlayerPacket %v err:%v", playerId, err)
			return false
		}
		err = db.GetPlayerDb().SaveComponentField(playerId, ComponentNamePendingMessages,
			util.Itoa(pendingMessage.MessageId), pendingMessageBytes)
		if err != nil {
			logger.Error("RoutePlayerPacket %v err:%v", playerId, err)
			return false
		}
		logger.Debug("save PendingMessage:%v playerId:%v cmd:%v", pendingMessage.MessageId, playerId, packet.Command())
	}
	conn := routeOpts.Connection
	if conn == nil {
		toServerId := routeOpts.ToServerId
		if toServerId == 0 {
			_, toServerId = cache.GetOnlinePlayer(playerId)
			if toServerId == 0 {
				logger.Error("RoutePlayerPacketErr player offline playerId:%v cmd:%v", playerId, packet.Command())
				return false
			}
		}
		conn = internal.GetServerList().GetServerConnection(toServerId)
	}
	var anyPacket *anypb.Any
	if packet.Message() != nil {
		var err error
		anyPacket, err = anypb.New(packet.Message())
		if err != nil {
			logger.Error("RoutePlayerPacketWithServer %v err:%v", playerId, err)
			return false
		}
	}
	routePacket := NewProtoPacketEx(pb.CmdServer_Cmd_RoutePlayerMessage, &pb.RoutePlayerMessage{
		Error:            errStr,
		ToPlayerId:       playerId,
		PacketCommand:    int32(packet.Command()),
		DirectSendClient: routeOpts.DirectSendClient,
		PendingMessageId: pendingMessageId,
		PacketData:       anyPacket,
	})
	if protoPacket, ok := packet.(*ProtoPacket); ok {
		routePacket.SetRpcCallId(protoPacket.RpcCallId())
	}
	return conn.SendPacket(routePacket)
}
