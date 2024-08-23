package network

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/gate"
	"github.com/fish-tennis/gserver/logger"
	"google.golang.org/protobuf/proto"
	"reflect"
)

// WebSocket客户端和gate之间的编解码
type WsClientCodec struct {
	SimpleProtoCodec
}

func NewWsClientCodec() *WsClientCodec {
	codec := &WsClientCodec{
		SimpleProtoCodec: *NewSimpleProtoCodec(),
	}
	return codec
}

func (this *WsClientCodec) Decode(connection Connection, data []byte) (newPacket Packet, err error) {
	if len(data) < SimplePacketHeaderSize {
		return nil, ErrPacketLength
	}
	packetHeader := &SimplePacketHeader{}
	packetHeader.ReadFrom(data)
	command := packetHeader.Command
	if protoMessageType, ok := this.MessageCreatorMap[PacketCommand(command)]; ok {
		if protoMessageType != nil {
			newProtoMessage := reflect.New(protoMessageType).Interface().(proto.Message)
			err = proto.Unmarshal(data[SimplePacketHeaderSize:], newProtoMessage)
			if err != nil {
				logger.Error("proto decode err:%v cmd:%v", err, command)
				return nil, err
			}
			return NewProtoPacket(PacketCommand(command), newProtoMessage), nil
		} else {
			// 支持只注册了消息号,没注册proto结构体的用法
			// support Register(command, nil), return the direct stream data to application layer
			return NewProtoPacketWithData(PacketCommand(command), data[SimplePacketHeaderSize:]), nil
		}
	}
	// 其他消息,gate直接转发,附加上playerId
	if clientData, ok := connection.GetTag().(*gate.ClientData); ok {
		return NewGatePacketWithData(clientData.PlayerId, PacketCommand(command), data[SimplePacketHeaderSize:]), nil
	}
	logger.Error("unSupport command:%v", command)
	return nil, ErrNotSupport
}
