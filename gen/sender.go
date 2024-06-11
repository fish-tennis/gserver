package gen

import (
	"github.com/fish-tennis/gnet"
	"google.golang.org/protobuf/proto"
)

type Sender interface {
	Send(command gnet.PacketCommand, message proto.Message, opts ...gnet.SendOption) bool

	SendPacket(packet gnet.Packet, opts ...gnet.SendOption) bool
}
