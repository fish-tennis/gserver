package game

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/pb"
)

// 玩家共用的处理接口
var _clientConnectionHandler *ClientConnectionHandler

// 客户端连接的handler
type ClientConnectionHandler struct {
	DefaultConnectionHandler
}

func NewClientConnectionHandler(protoCodec *ProtoCodec) *ClientConnectionHandler {
	_clientConnectionHandler = &ClientConnectionHandler{
		DefaultConnectionHandler: *NewDefaultConnectionHandler(protoCodec),
	}
	return _clientConnectionHandler
}

func (this *ClientConnectionHandler) OnRecvPacket(connection Connection, packet Packet) {
	if connection.GetTag() != nil && packet.Command() != PacketCommand(pb.CmdInner_Cmd_HeartBeatReq) {
		player := GetPlayer(connection.GetTag().(int64))
		if player != nil {
			// 在线玩家的消息,转到玩家消息处理协程去处理
			if protoPacket, ok := packet.(*ProtoPacket); ok {
				player.OnRecvPacket(protoPacket)
				return
			}
		}
	}
	// 未登录的玩家消息,走默认处理,在收包协程里
	this.DefaultConnectionHandler.OnRecvPacket(connection, packet)
}
