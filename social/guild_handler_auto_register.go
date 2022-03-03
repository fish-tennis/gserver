package social

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/gameplayer"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
)

func GuildHandlerAutoRegister(handler PacketHandlerRegister, playerMgr gameplayer.PlayerMgr) {
	logger.Debug("GuildHandlerAutoRegister")
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
		return new(pb.GuildCreateReq)
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

	// 其他服务器转发过来的公会消息
	handler.Register(PacketCommand(pb.CmdRoute_Cmd_GuildRoutePlayerMessageReq), func(connection Connection, packet *ProtoPacket) {
		req := packet.Message().(*pb.GuildRoutePlayerMessageReq)
		// 再验证一次是否属于本服务器管理
		if GuildRoute(req.FromGuildId) != internal.GetServer().GetServerId() {
			logger.Error("wrong guildid:%v", req.FromGuildId)
			return
		}
		// 先从内存中查找
		guild := GetGuildById(req.FromGuildId)
		if guild == nil {
			// 再到数据库加载
			guild = LoadGuild(req.FromGuildId)
			if guild == nil {
				logger.Error("not find guild:%v", guild)
				return
			}
		}
		message,err := req.PacketData.UnmarshalNew()
		if err != nil {
			logger.Error("UnmarshalNew %v err: %v", req.FromGuildId, err)
			return
		}
		err = req.PacketData.UnmarshalTo(message)
		if err != nil {
			logger.Error("UnmarshalTo %v err: %v", req.FromGuildId, err)
			return
		}
		guild.PushMessage(&GuildMessage{
			fromPlayerId: req.FromPlayerId,
			fromServerId: req.FromServerId,
			cmd: PacketCommand(uint16(req.PacketCommand)),
			message: message,
		})
	}, func() proto.Message {
		return new(pb.RequestGuildDataReq)
	})

}