package social

import (
	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

func GuildServerHandlerRegister(handler PacketHandlerRegister, playerMgr gentity.PlayerMgr) {
	// 其他服务器转发过来的公会消息
	handler.Register(PacketCommand(pb.CmdRoute_Cmd_GuildRoutePlayerMessageReq), func(connection Connection, packet *ProtoPacket) {
		req := packet.Message().(*pb.GuildRoutePlayerMessageReq)
		// 再验证一次是否属于本服务器管理
		if GuildRoute(req.FromGuildId) != gentity.GetServer().GetServerId() {
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
			fromPlayerName: req.FromPlayerName,
			cmd: PacketCommand(uint16(req.PacketCommand)),
			message: message,
		})
	}, new(pb.GuildRoutePlayerMessageReq))
}