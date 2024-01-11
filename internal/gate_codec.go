package internal

import (
	"encoding/binary"
	. "github.com/fish-tennis/gnet"
	"google.golang.org/protobuf/proto"
	"reflect"
	"github.com/fish-tennis/gserver/logger"
)

// gate和其他服务器之间的编解码
type GateCodec struct {
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

func NewGateCodec(protoMessageTypeMap map[PacketCommand]reflect.Type) *GateCodec {
	codec := &GateCodec{
		RingBufferCodec:   RingBufferCodec{},
		MessageCreatorMap: protoMessageTypeMap,
	}
	if codec.MessageCreatorMap == nil {
		codec.MessageCreatorMap = make(map[PacketCommand]reflect.Type)
	}
	codec.DataEncoder = codec.EncodePacket
	codec.DataDecoder = codec.DecodePacket
	return codec
}

// 注册消息和proto.Message的映射
//
//	protoMessage can be nil
func (this *GateCodec) Register(command PacketCommand, protoMessage proto.Message) {
	if protoMessage == nil {
		this.MessageCreatorMap[command] = nil
		return
	}
	this.MessageCreatorMap[command] = reflect.TypeOf(protoMessage).Elem()
}

func (this *GateCodec) EncodePacket(connection Connection, packet Packet) [][]byte {
	gatePacket,_ := packet.(*GatePacket)
	protoMessage := packet.Message()
	// 先写入消息号
	// write PacketCommand
	commandBytes := make([]byte, 10)
	binary.LittleEndian.PutUint16(commandBytes, uint16(packet.Command()))
	// write PlayerId
	binary.LittleEndian.PutUint64(commandBytes[2:], uint64(gatePacket.PlayerId()))
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

func (this *GateCodec) DecodePacket(connection Connection, packetHeader PacketHeader, packetData []byte) Packet {
	decodedPacketData := packetData
	// Q:这里可以对packetData进行解码,如异或,解密,解压等
	// you can decode packetData here, such as XOR, encryption, compression, etc
	if this.ProtoPacketBytesDecoder != nil {
		decodedPacketData = this.ProtoPacketBytesDecoder(packetData)
	}
	if len(decodedPacketData) < 10 {
		return nil
	}
	command := binary.LittleEndian.Uint16(decodedPacketData[:2])
	playerId := int64(binary.LittleEndian.Uint64(decodedPacketData[2:10]))
	if protoMessageType, ok := this.MessageCreatorMap[PacketCommand(command)]; ok {
		if protoMessageType != nil {
			newProtoMessage := reflect.New(protoMessageType).Interface().(proto.Message)
			err := proto.Unmarshal(decodedPacketData[10:], newProtoMessage)
			if err != nil {
				logger.Error("proto decode err:%v cmd:%v", err, command)
				return nil
			}
			return &GatePacket{
				command: PacketCommand(command),
				playerId: playerId,
				message: newProtoMessage,
			}
		} else {
			// 支持只注册了消息号,没注册proto结构体的用法
			// support Register(command, nil), return the direct stream data to application layer
			return &GatePacket{
				command: PacketCommand(command),
				playerId: playerId,
				data:    decodedPacketData[10:],
			}
		}
	}
	return &GatePacket{
		command: PacketCommand(command),
		playerId: playerId,
		data:    decodedPacketData[10:],
	}
}
