package social

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/gameplayer"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
)

func GuildClientHandlerRegister(handler PacketHandlerRegister, playerMgr gameplayer.PlayerMgr) {
	logger.Debug("GuildClientHandlerRegister")
	handler.Register(PacketCommand(pb.CmdGuild_Cmd_GuildListReq), func(connection Connection, packet *ProtoPacket) {
		if connection.GetTag() != nil {
			player := playerMgr.GetPlayer(connection.GetTag().(int64))
			if player != nil {
				OnGuildListReq(player, packet.Message().(*pb.GuildListReq))
				player.SaveCache()
			}
		}
	}, func() proto.Message {
		return new(pb.GuildListReq)
	})

	handler.Register(PacketCommand(pb.CmdGuild_Cmd_GuildCreateReq), func(connection Connection, packet *ProtoPacket) {
		if connection.GetTag() != nil {
			player := playerMgr.GetPlayer(connection.GetTag().(int64))
			if player != nil {
				OnGuildCreateReq(player, packet.Message().(*pb.GuildCreateReq))
				player.SaveCache()
			}
		}
	}, func() proto.Message {
		return new(pb.GuildCreateReq)
	})

	handler.Register(PacketCommand(pb.CmdGuild_Cmd_GuildJoinReq), func(connection Connection, packet *ProtoPacket) {
		if connection.GetTag() != nil {
			player := playerMgr.GetPlayer(connection.GetTag().(int64))
			if player != nil {
				if player.GetGuild().GetGuildData().GuildId > 0 {
					logger.Error("CantJoinGuild %v", player.GetId())
					return
				}
				req := packet.Message().(*pb.GuildJoinReq)
				GuildRouteReqPacket(player, req.Id, packet)
			}
		}
	}, func() proto.Message {
		return new(pb.GuildJoinReq)
	})

	handler.Register(PacketCommand(pb.CmdGuild_Cmd_RequestGuildDataReq), func(connection Connection, packet *ProtoPacket) {
		if connection.GetTag() != nil {
			player := playerMgr.GetPlayer(connection.GetTag().(int64))
			if player != nil {
				guildId := player.GetGuild().GetGuildData().GuildId
				if guildId == 0 {
					logger.Error("no guild %v", player.GetId())
					return
				}
				GuildRouteReqPacket(player, guildId, packet)
			}
		}
	}, func() proto.Message {
		return new(pb.RequestGuildDataReq)
	})



}