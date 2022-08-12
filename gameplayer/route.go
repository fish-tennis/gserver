package gameplayer

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// 路由玩家消息
// 如果目标玩家在本服务器,则直接路由到本服务器上的玩家
// 如果目标玩家在另一个服务器上,则转发到目标服务器
// directSendClient true:消息直接发给客户端 false:放入玩家消息队列,消息将在玩家协程中被处理
func RoutePlayerPacket(playerId int64, cmd PacketCommand, message proto.Message, directSendClient bool) bool {
	player := GetPlayerMgr().GetPlayer(playerId)
	if player != nil {
		if directSendClient {
			return player.Send(cmd, message)
		} else {
			player.OnRecvPacket(NewProtoPacket(cmd, message))
		}
		return true
	}
	_,serverId := cache.GetOnlinePlayer(playerId)
	if serverId == 0 {
		return false
	}
	if serverId == internal.GetServer().GetServerId() {
		return false
	}
	return RoutePlayerPacketWithServer(playerId, serverId, cmd, message, directSendClient)
}

// 路由玩家消息到目标服务器
// 如果是本服务器,则直接路由到本服务器上的玩家
func RoutePlayerPacketWithServer(playerId int64, serverId int32, cmd PacketCommand, message proto.Message, directSendClient bool) bool {
	if serverId == internal.GetServer().GetServerId() {
		player := GetPlayerMgr().GetPlayer(playerId)
		if player != nil {
			if directSendClient {
				return player.Send(cmd, message)
			} else {
				player.OnRecvPacket(NewProtoPacket(cmd, message))
			}
			return true
		}
		return false
	}
	any,err := anypb.New(message)
	if err != nil {
		logger.Error("RoutePlayerPacketWithServer %v err:%v", playerId, err)
		return false
	}
	return internal.GetServerList().SendToServer(serverId, PacketCommand(pb.CmdRoute_Cmd_RoutePlayerMessage), &pb.RoutePlayerMessage{
		ToPlayerId: playerId,
		PacketCommand: int32(cmd),
		DirectSendClient: directSendClient,
		PacketData: any,
	})
}
