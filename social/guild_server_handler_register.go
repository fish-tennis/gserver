package social

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
)

func GuildServerHandlerRegister(handler PacketHandlerRegister) {
	slog.Info("GuildServerHandlerRegister")
	// 其他服务器转发过来的公会消息
	handler.Register(PacketCommand(pb.CmdRoute_Cmd_GuildRoutePlayerMessageReq), func(connection Connection, packet Packet) {
		req := packet.Message().(*pb.GuildRoutePlayerMessageReq)
		slog.Debug("GuildRoutePlayerMessageReq", "packet", req)
		_guildMgr.ParseRoutePacket(connection, packet, req.FromGuildId)
	}, new(pb.GuildRoutePlayerMessageReq))
}
