package gameplayer

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gentity/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// 路由参数
type RouteOptions struct {
	// true:消息直接发给客户端
	// false:放入玩家消息队列,消息将在玩家协程中被处理
	DirectSendClient bool

	// 路由到指定的服务器
	ToServerId int32

	// 先保存到数据库(player.pendingmessages),防止路由失败造成消息丢失
	SaveDb bool
}

func NewRouteOptions() *RouteOptions {
	return &RouteOptions{}
}

func DirectSendClientRouteOptions() *RouteOptions {
	return &RouteOptions{
		DirectSendClient: true,
	}
}

func SaveDbRouteOptions() *RouteOptions {
	return &RouteOptions{
		SaveDb: true,
	}
}

// set DirectSendClient
func (this *RouteOptions) SetDirectSendClient(directSendClient bool) *RouteOptions {
	this.DirectSendClient = directSendClient
	return this
}

// set ToServerId
func (this *RouteOptions) SetToServerId(toServerId int32) *RouteOptions {
	this.ToServerId = toServerId
	return this
}

// 路由玩家消息
// 如果目标玩家在本服务器,则直接路由到本服务器上的玩家
// 如果目标玩家在另一个服务器上,则转发到目标服务器 ServerA -> ServerB -> Player
//
// DirectSendClientRouteOptions():
// 消息直接转发给客户端,不做逻辑处理 ServerA -> ServerB -> Client
//
// 举例:
// 有人申请加入公会,公会广播该消息给公会成员,ServerB收到消息后,直接把消息发给客户端(Player.Send),而不需要放入玩家的逻辑消息队列(Player.OnRecvPacket)
//
// SaveDbRouteOptions(): 消息先保存数据库再转发,防止丢失
// 举例:
// 公会会长同意了玩家A的入会申请,此时玩家A可能不在线,就把该消息存入玩家的数据库,待玩家下次上线时,从数据库取出该消息,并进行相应的逻辑处理
func RoutePlayerPacket(playerId int64, cmd PacketCommand, message proto.Message, opts ...*RouteOptions) bool {
	var (
		directSendClient = false
		toServerId = int32(0)
		saveDb = false
	)
	for _,opt := range opts {
		directSendClient = opt.DirectSendClient
		//toServerId = opt.ToServerId
		saveDb = opt.SaveDb
	}
	player := GetPlayerMgr().GetPlayer(playerId)
	if player != nil {
		if directSendClient {
			return player.Send(cmd, message)
		} else {
			player.OnRecvPacket(NewProtoPacket(cmd, message))
		}
		return true
	}
	if saveDb {
		any,err := anypb.New(message)
		if err != nil {
			logger.Error("RoutePlayerPacket %v err:%v", playerId, err)
			return false
		}
		routePacket := &pb.RoutePlayerMessage{
			ToPlayerId: playerId,
			PacketCommand: int32(cmd),
			DirectSendClient: false,
			MessageId: util.GenUniqueId(), // 消息号生成唯一id
			PacketData: any,
		}
		routePacketBytes,err := proto.Marshal(routePacket)
		if err != nil {
			logger.Error("RoutePlayerPacket %v err:%v", playerId, err)
			return false
		}
		err = db.GetPlayerDb().SaveComponentField(playerId, "pendingmessages", util.Itoa(routePacket.MessageId), routePacketBytes)
		if err != nil {
			logger.Error("RoutePlayerPacket %v err:%v", playerId, err)
			return false
		}
	}
	if toServerId == 0 {
		_,toServerId = cache.GetOnlinePlayer(playerId)
		if toServerId == 0 {
			return false
		}
		if toServerId == internal.GetServer().GetServerId() {
			return false
		}
	}
	any,err := anypb.New(message)
	if err != nil {
		logger.Error("RoutePlayerPacketWithServer %v err:%v", playerId, err)
		return false
	}
	return internal.GetServerList().SendToServer(toServerId, PacketCommand(pb.CmdRoute_Cmd_RoutePlayerMessage), &pb.RoutePlayerMessage{
		ToPlayerId: playerId,
		PacketCommand: int32(cmd),
		DirectSendClient: directSendClient,
		PacketData: any,
	})
}
