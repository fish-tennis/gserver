package social

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/game"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"reflect"
)

func GuildClientHandlerRegister(handler PacketHandlerRegister, playerMgr gentity.PlayerMgr) {
	logger.Debug("GuildClientHandlerRegister")
	handler.Register(PacketCommand(pb.CmdGuild_Cmd_GuildListReq), func(connection Connection, packet *ProtoPacket) {
		if connection.GetTag() != nil {
			player := playerMgr.GetPlayer(connection.GetTag().(int64))
			if player != nil {
				gamePlayer := player.(*game.Player)
				OnGuildListReq(gamePlayer, packet.Message().(*pb.GuildListReq))
				gamePlayer.SaveCache(cache.Get())
			}
		}
	}, new(pb.GuildListReq))

	handler.Register(PacketCommand(pb.CmdGuild_Cmd_GuildCreateReq), func(connection Connection, packet *ProtoPacket) {
		if connection.GetTag() != nil {
			player := playerMgr.GetPlayer(connection.GetTag().(int64))
			if player != nil {
				gamePlayer := player.(*game.Player)
				OnGuildCreateReq(gamePlayer, packet.Message().(*pb.GuildCreateReq))
				gamePlayer.SaveCache(cache.Get())
			}
		}
	}, new(pb.GuildCreateReq))

	handler.Register(PacketCommand(pb.CmdGuild_Cmd_GuildJoinReq), func(connection Connection, packet *ProtoPacket) {
		if connection.GetTag() != nil {
			player := playerMgr.GetPlayer(connection.GetTag().(int64))
			if player != nil {
				gamePlayer := player.(*game.Player)
				if gamePlayer.GetGuild().GetGuildData().GuildId > 0 {
					logger.Error("CantJoinGuild %v", gamePlayer.GetId())
					return
				}
				req := packet.Message().(*pb.GuildJoinReq)
				GuildRouteReqPacket(gamePlayer, req.Id, packet)
			}
		}
	}, new(pb.GuildJoinReq))

	RegisterPlayerGuildHandler(handler, playerMgr, new(pb.GuildDataViewReq))
	RegisterPlayerGuildHandler(handler, playerMgr, new(pb.GuildJoinAgreeReq))
}

// 注册玩家公会消息回调
func RegisterPlayerGuildHandler(handler PacketHandlerRegister, playerMgr gentity.PlayerMgr, message proto.Message) {
	messageName := message.ProtoReflect().Descriptor().Name()
	cmd := util.GetMessageIdByComponentMessageName("gserver", "Guild", string(messageName))
	handler.Register(PacketCommand(uint16(cmd)), func(connection Connection, packet *ProtoPacket) {
		if connection.GetTag() != nil {
			player := playerMgr.GetPlayer(connection.GetTag().(int64))
			if player != nil {
				gamePlayer := player.(*game.Player)
				guildId := gamePlayer.GetGuild().GetGuildData().GuildId
				if guildId == 0 {
					logger.Error("no guild %v", gamePlayer.GetId())
					return
				}
				GuildRouteReqPacket(gamePlayer, guildId, packet)
			}
		}
	}, reflect.New(reflect.TypeOf(message).Elem()).Interface().(proto.Message))
	logger.Debug("RegisterPlayerGuildHandler %v->%v", cmd, messageName)
}
