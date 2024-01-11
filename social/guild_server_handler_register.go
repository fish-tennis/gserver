package social

import (
	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/pb"
)

func GuildServerHandlerRegister(handler PacketHandlerRegister, playerMgr gentity.PlayerMgr) {
	// 其他服务器转发过来的公会消息
	handler.Register(PacketCommand(pb.CmdRoute_Cmd_GuildRoutePlayerMessageReq), func(connection Connection, packet Packet) {
		req := packet.Message().(*pb.GuildRoutePlayerMessageReq)
		_guildMgr.ParseRoutePacket(req.FromGuildId, packet)
	}, new(pb.GuildRoutePlayerMessageReq))
}