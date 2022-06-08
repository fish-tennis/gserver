package social

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/gameplayer"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"google.golang.org/protobuf/proto"
	"reflect"
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
	}, new(pb.GuildListReq))

	handler.Register(PacketCommand(pb.CmdGuild_Cmd_GuildCreateReq), func(connection Connection, packet *ProtoPacket) {
		if connection.GetTag() != nil {
			player := playerMgr.GetPlayer(connection.GetTag().(int64))
			if player != nil {
				OnGuildCreateReq(player, packet.Message().(*pb.GuildCreateReq))
				player.SaveCache()
			}
		}
	}, new(pb.GuildCreateReq))

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
	}, new(pb.GuildJoinReq))

	RegisterPlayerGuildHandler(handler, playerMgr, new(pb.GuildDataViewReq))
	RegisterPlayerGuildHandler(handler, playerMgr, new(pb.GuildJoinAgreeReq))
}

// 注册玩家公会消息回调
func RegisterPlayerGuildHandler(handler PacketHandlerRegister, playerMgr gameplayer.PlayerMgr, message proto.Message) {
	messageName := message.ProtoReflect().Descriptor().Name()
	cmd := util.GetMessageIdByMessageName("gserver", "Guild", string(messageName))
	handler.Register(PacketCommand(uint16(cmd)), func(connection Connection, packet *ProtoPacket) {
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
	}, reflect.New(reflect.TypeOf(message).Elem()).Interface().(proto.Message))
	logger.Debug("RegisterPlayerGuildHandler %v->%v", cmd, messageName)
}
