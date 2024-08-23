package network

import (
	"encoding/binary"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/logger"
	"google.golang.org/protobuf/proto"
	"reflect"
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

func (this *GateCodec) EncodePacket(connection Connection, packet Packet) ([][]byte, uint8) {
	gatePacket, _ := packet.(*GatePacket)
	protoMessage := packet.Message()
	headerFlags := uint8(0)
	// 先写入消息号
	// write PacketCommand
	commandBytes := make([]byte, 10)
	binary.LittleEndian.PutUint16(commandBytes, uint16(packet.Command()))
	// write PlayerId
	binary.LittleEndian.PutUint64(commandBytes[2:], uint64(gatePacket.PlayerId()))
	rpcCallId := gatePacket.rpcCallId
	var rpcCallIdBytes []byte
	// rpcCall才会写入rpcCallId
	if rpcCallId > 0 {
		rpcCallIdBytes = make([]byte, 4)
		binary.LittleEndian.PutUint32(rpcCallIdBytes, rpcCallId)
		headerFlags = RpcCall
		//logger.Debug("write rpcCallId:%v", rpcCallId)
	}
	var messageBytes []byte
	if protoMessage != nil {
		var err error
		messageBytes, err = proto.Marshal(protoMessage)
		if err != nil {
			logger.Error("proto encode err:%v cmd:%v", err, packet.Command())
			return nil, 0
		}
	} else {
		// 支持提前序列化好的数据
		// support direct encoded data from application layer
		messageBytes = packet.GetStreamData()
	}
	// 这里可以继续对messageBytes进行编码,如异或,加密,压缩等
	// you can continue to encode messageBytes here, such as XOR, encryption, compression, etc
	if this.ProtoPacketBytesEncoder != nil {
		if rpcCallId > 0 {
			return this.ProtoPacketBytesEncoder([][]byte{commandBytes, rpcCallIdBytes, messageBytes}), headerFlags
		}
		return this.ProtoPacketBytesEncoder([][]byte{commandBytes, messageBytes}), headerFlags
	}
	if rpcCallId > 0 {
		return [][]byte{commandBytes, rpcCallIdBytes, messageBytes}, headerFlags
	}
	return [][]byte{commandBytes, messageBytes}, headerFlags
}

func (this *GateCodec) DecodePacket(connection Connection, packetHeader PacketHeader, packetData []byte) Packet {
	decodedPacketData := packetData
	// Q:这里可以对packetData进行解码,如异或,解密,解压等
	// you can decode packetData here, such as XOR, decryption, decompression, etc
	if this.ProtoPacketBytesDecoder != nil {
		decodedPacketData = this.ProtoPacketBytesDecoder(packetData)
	}
	// command:2 playerId:8
	if len(decodedPacketData) < 10 {
		return nil
	}
	isRpcCall := false
	if defaultPacketHeader, ok := packetHeader.(*DefaultPacketHeader); ok {
		isRpcCall = defaultPacketHeader.HasFlag(RpcCall)
	}
	// command:2 playerId:8 rpcCallId:4
	if isRpcCall && len(decodedPacketData) < 14 {
		return nil
	}
	command := binary.LittleEndian.Uint16(decodedPacketData[:2])
	playerId := int64(binary.LittleEndian.Uint64(decodedPacketData[2:10]))
	offset := 10
	rpcCallId := uint32(0)
	if isRpcCall {
		rpcCallId = binary.LittleEndian.Uint32(decodedPacketData[offset : offset+4])
		offset += 4
		//logger.Debug("read rpcCallId:%v", rpcCallId)
	}
	if protoMessageType, ok := this.MessageCreatorMap[PacketCommand(command)]; ok {
		if protoMessageType != nil {
			newProtoMessage := reflect.New(protoMessageType).Interface().(proto.Message)
			err := proto.Unmarshal(decodedPacketData[offset:], newProtoMessage)
			if err != nil {
				logger.Error("proto decode err:%v cmd:%v", err, command)
				return nil
			}
			return &GatePacket{
				command:   PacketCommand(command),
				playerId:  playerId,
				rpcCallId: rpcCallId,
				message:   newProtoMessage,
			}
		} else {
			// 支持只注册了消息号,没注册proto结构体的用法
			// support Register(command, nil), return the direct stream data to application layer
			return &GatePacket{
				command:   PacketCommand(command),
				playerId:  playerId,
				rpcCallId: rpcCallId,
				data:      decodedPacketData[offset:],
			}
		}
	}
	// 允许消息不注册,留给业务层解析
	return &GatePacket{
		command:   PacketCommand(command),
		playerId:  playerId,
		rpcCallId: rpcCallId,
		data:      decodedPacketData[offset:],
	}
}
