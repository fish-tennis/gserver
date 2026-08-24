package internal

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"log/slog"
)


// 路由源头消息
type RouteSourcePacket struct {
	FromPlayerId   int64
	FromServerId   int32
	FromPlayerName string
	Cmd            gnet.PacketCommand
	Message        proto.Message
	SrcPacket      gnet.Packet     // 来源packet
	SrcConnection  gnet.Connection // 来源连接,返回消息"原路返回",才能实现rpc功能;内部事件时携带触发事件的连接
	IsDirectClient bool            // 是否来自直连客户端
	InternalEvent  int             // 内部事件类型(0=普通消息),非0时 Message 为 nil,OnProcessMessage 拦截后交给 handleInternalEvent
}

// 根据公会id查找对应的服务器
// 公会实体由social服务器承载,路由到social服务器
func RouteGuildServerId(guildId int64) int32 {
	servers := GetServerList().GetServersByType(ServerType_Social)
	if len(servers) == 0 {
		return 0
	}
	// 这里只演示了最简单的路由方式
	index := guildId % int64(len(servers))
	if index < 0 {
		index += int64(len(servers))
	}
	return servers[index].GetServerId()
}

// 玩家对公会的请求消息转换成路由消息
//
//	原始消息基础上再加上一些附加数据
//	client -> game.Guild -> social.Guild
func PacketToGuildRoutePacket(fromPlayerId int64, fromPlayerName string, reqPacket gnet.Packet, guildId int64) gnet.Packet {
	anyPacket, err := anypb.New(reqPacket.Message())
	if err != nil {
		slog.Error("PacketToGuildRoutePacket anypb error", "error", err)
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
	return network.NewPacket(routePacket)
}

// 玩家的请求消息转换成路由消息
//
//	原始消息基础上再加上一些附加数据
//	client -> PlayerComponent(game) -> 其他服或其他协程
func PacketToTargetRoutePacket(fromPlayerId int64, fromPlayerName string, req proto.Message, targetEntityId int64) gnet.Packet {
	anyPacket, err := anypb.New(req)
	if err != nil {
		slog.Error("PacketToTargetRoutePacketErr", "error", err, "fromPlayerId", fromPlayerId)
		return nil
	}
	routePacket := &pb.RoutePlayerMessageReq{
		FromPlayerId: fromPlayerId,
		//FromGuildId:    0,
		FromServerId:   gentity.GetApplication().GetId(),
		FromPlayerName: fromPlayerName,
		PacketCommand:  network.GetCommandByProto(req),
		PacketData:     anyPacket,
		TargetEntityId: targetEntityId,
	}
	return network.NewPacket(routePacket)
}
