package network

import (
	. "github.com/fish-tennis/gnet"
	"google.golang.org/protobuf/proto"
)

type GatePacket struct {
	command  PacketCommand
	playerId int64
	// use for rpc call
	rpcCallId uint32
	errorCode uint32
	message   proto.Message
	data      []byte
}

func NewGatePacket(playerId int64, command PacketCommand, message proto.Message) *GatePacket {
	if command == 0 {
		command = PacketCommand(GetCommandByProto(message))
	}
	return &GatePacket{
		command:  command,
		playerId: playerId,
		message:  message,
	}
}

func NewGatePacketWithData(playerId int64, command PacketCommand, data []byte) *GatePacket {
	return &GatePacket{
		command:  command,
		playerId: playerId,
		data:     data,
	}
}

func (this *GatePacket) Command() PacketCommand {
	return this.command
}

func (this *GatePacket) PlayerId() int64 {
	return this.playerId
}

func (this *GatePacket) SetPlayerId(playerId int64) {
	this.playerId = playerId
}

func (this *GatePacket) Message() proto.Message {
	return this.message
}

func (this *GatePacket) RpcCallId() uint32 {
	return this.rpcCallId
}

func (this *GatePacket) SetRpcCallId(rpcCallId uint32) {
	this.rpcCallId = rpcCallId
}

func (this *GatePacket) ErrorCode() uint32 {
	return this.errorCode
}

func (this *GatePacket) SetErrorCode(code uint32) *GatePacket {
	this.errorCode = code
	return this
}

func (this *GatePacket) WithStreamData(streamData []byte) *GatePacket {
	this.data = streamData
	return this
}

func (this *GatePacket) WithRpc(arg any) *GatePacket {
	switch v := arg.(type) {
	case uint32:
		this.rpcCallId = v
	case Packet:
		this.rpcCallId = v.RpcCallId()
	}
	return this
}

// 某些特殊需求会直接使用序列化好的数据
//
//	support stream data
func (this *GatePacket) GetStreamData() []byte {
	return this.data
}

// deep copy
func (this *GatePacket) Clone() Packet {
	newPacket := &GatePacket{
		command:   this.command,
		rpcCallId: this.rpcCallId,
		errorCode: this.errorCode,
		playerId:  this.playerId,
		message:   proto.Clone(this.message),
	}
	if len(this.data) > 0 {
		newPacket.data = make([]byte, len(this.data))
		copy(newPacket.data, this.data)
	}
	return newPacket
}

func (this *GatePacket) ToProtoPacket() *ProtoPacket {
	var p *ProtoPacket
	if this.Message() != nil {
		p = NewProtoPacket(this.Command(), this.Message())
	} else {
		p = NewProtoPacketWithData(this.Command(), this.GetStreamData())
	}
	p.SetRpcCallId(this.rpcCallId)
	p.SetErrorCode(this.errorCode)
	return p
}

func IsGatePacket(packet Packet) bool {
	_, ok := packet.(*GatePacket)
	return ok
}

// 根据请求消息的类型,自动适配不同的发消息接口
func SendPacketAdapt(connection Connection, reqPacket Packet, sendMessage proto.Message) bool {
	return SendPacketAdaptWithError(connection, reqPacket, sendMessage, 0)
}

func SendPacketAdaptWithError(connection Connection, reqPacket Packet, sendMessage proto.Message, errorCode int32) bool {
	cmd := GetCommandByProto(sendMessage)
	if gatePacket, ok := reqPacket.(*GatePacket); ok {
		return connection.SendPacket(NewGatePacket(gatePacket.PlayerId(), PacketCommand(cmd), sendMessage).SetErrorCode(uint32(errorCode)))
	} else {
		packet := NewProtoPacket(PacketCommand(cmd), sendMessage).SetErrorCode(uint32(errorCode))
		return connection.SendPacket(packet)
	}
}
