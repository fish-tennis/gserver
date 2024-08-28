package network

import (
	"github.com/fish-tennis/gnet"
	"google.golang.org/protobuf/proto"
)

func NewPacket(message proto.Message) *gnet.ProtoPacket {
	cmd := GetCommandByProto(message)
	return gnet.NewProtoPacketEx(cmd, message)
}
