package gate

import (
	"encoding/binary"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"google.golang.org/protobuf/proto"
	"reflect"
)

// 客户端和gate之间的编解码
type ClientCodec struct {
	RingBufferCodec

	// 在proto序列化后的数据,再做一层编码
	// encoder after proto.Message serialize
	ProtoPacketBytesEncoder func(protoPacketBytes [][]byte) [][]byte

	// 在proto反序列化之前,先做一层解码
	// decoder before proto.Message deserialize
	ProtoPacketBytesDecoder func(packetData []byte) []byte

	// 消息号和proto.Message type的映射表
	MessageCreatorMap map[PacketCommand]reflect.Type
}

func NewClientCodec() *ClientCodec {
	codec := &ClientCodec{
		RingBufferCodec:   RingBufferCodec{},
		MessageCreatorMap: make(map[PacketCommand]reflect.Type),
	}
	codec.DataEncoder = codec.EncodePacket
	codec.DataDecoder = codec.DecodePacket
	return codec
}

// 注册消息和proto.Message的映射
//  protoMessage can be nil
func (this *ClientCodec) Register(command PacketCommand, protoMessage proto.Message) {
	if protoMessage == nil {
		this.MessageCreatorMap[command] = nil
		return
	}
	this.MessageCreatorMap[command] = reflect.TypeOf(protoMessage).Elem()
}

func (this *ClientCodec) EncodePacket(connection Connection, packet Packet) [][]byte {
	protoMessage := packet.Message()
	// 先写入消息号
	// write PacketCommand
	commandBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(commandBytes, uint16(packet.Command()))
	var messageBytes []byte
	if protoMessage != nil {
		var err error
		messageBytes, err = proto.Marshal(protoMessage)
		if err != nil {
			logger.Error("proto encode err:%v cmd:%v", err, packet.Command())
			return nil
		}
	} else {
		// 支持提前序列化好的数据
		// support direct encoded data from application layer
		messageBytes = packet.GetStreamData()
	}
	// 这里可以继续对messageBytes进行编码,如异或,加密,压缩等
	// you can continue to encode messageBytes here, such as XOR, encryption, compression, etc
	if this.ProtoPacketBytesEncoder != nil {
		return this.ProtoPacketBytesEncoder([][]byte{commandBytes, messageBytes})
	}
	return [][]byte{commandBytes, messageBytes}
}

func (this *ClientCodec) DecodePacket(connection Connection, packetHeader PacketHeader, packetData []byte) Packet {
	decodedPacketData := packetData
	// Q:这里可以对packetData进行解码,如异或,解密,解压等
	// you can decode packetData here, such as XOR, encryption, compression, etc
	if this.ProtoPacketBytesDecoder != nil {
		decodedPacketData = this.ProtoPacketBytesDecoder(packetData)
	}
	if len(decodedPacketData) < 2 {
		return nil
	}
	command := binary.LittleEndian.Uint16(decodedPacketData[:2])
	if protoMessageType, ok := this.MessageCreatorMap[PacketCommand(command)]; ok {
		// 有一些客户端消息,是gate处理
		if protoMessageType != nil {
			newProtoMessage := reflect.New(protoMessageType).Interface().(proto.Message)
			err := proto.Unmarshal(decodedPacketData[2:], newProtoMessage)
			if err != nil {
				logger.Error("proto decode err:%v cmd:%v", err, command)
				return nil
			}
			return NewProtoPacket(PacketCommand(command), newProtoMessage)
		} else {
			// 支持只注册了消息号,没注册proto结构体的用法
			// support Register(command, nil), return the direct stream data to application layer
			return NewProtoPacketWithData(PacketCommand(command), decodedPacketData[2:])
		}
	}
	// 其他消息,gate直接转发,附加上playerId
	if clientData,ok := connection.GetTag().(*ClientData); ok {
		return internal.NewGatePacketWithData(clientData.PlayerId, PacketCommand(command), decodedPacketData[2:])
	}
	logger.Error("unSupport command:%v", command)
	return nil
}
