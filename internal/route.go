package internal

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/types/known/anypb"
)

// 根据公会id查找对应的服务器
func RouteGuildServerId(guildId int64) int32 {
	servers := GetServerList().GetServersByType(ServerType_Game)
	if len(servers) == 0 {
		return 0
	}
	// 这里只演示了最简单的路由方式
	index := guildId % int64(len(servers))
	return servers[index].GetServerId()
}

// 玩家对公会的请求消息转换成路由消息
//
//	原始消息基础上再加上一些附加数据
//	client -> game.Guild -> social.Guild
func PacketToGuildRoutePacket(fromPlayerId int64, fromPlayerName string, reqPacket gnet.Packet, guildId int64) gnet.Packet {
	anyPacket, err := anypb.New(reqPacket.Message())
	if err != nil {
		logger.Error("PacketToGuildRoutePacket anypb err:%v", err)
		return nil
	}
	routePacket := &pb.GuildRoutePlayerMessageReq{
		FromPlayerId:   fromPlayerId,
		FromGuildId:    guildId,
		FromServerId:   gentity.GetApplication().GetId(),
		FromPlayerName: fromPlayerName,
		PacketCommand:  int32(reqPacket.Command()),
		PacketData:     anyPacket,
	}
	return gnet.NewProtoPacketEx(pb.CmdServer_Cmd_GuildRoutePlayerMessageReq, routePacket)
}
