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
	message   proto.Message
	data      []byte
}

func NewGatePacket(playerId int64, command PacketCommand, message proto.Message) *GatePacket {
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

func (this *GatePacket) WithStreamData(streamData []byte) *GatePacket {
	this.data = streamData
	return this
}

func (this *GatePacket) WithRpc(arg any) *GatePacket {
	switch v := arg.(type) {
	case uint32:
		this.rpcCallId = v
	case RpcCallIdSetter:
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
		command:  this.command,
		playerId: this.playerId,
		message:  proto.Clone(this.message),
	}
	if len(this.data) > 0 {
		newPacket.data = make([]byte, len(this.data))
		copy(newPacket.data, this.data)
	}
	return newPacket
}

func (this *GatePacket) ToProtoPacket() *ProtoPacket {
	if this.Message() != nil {
		return NewProtoPacket(this.Command(), this.Message())
	} else {
		return NewProtoPacketWithData(this.Command(), this.GetStreamData())
	}
}

func IsGatePacket(packet Packet) bool {
	_, ok := packet.(*GatePacket)
	return ok
}

// 根据请求消息的类型,自动适配不同的发消息接口
func SendPacketAdapt(connection Connection, reqPacket Packet, command PacketCommand, message proto.Message) bool {
	if gatePacket, ok := reqPacket.(*GatePacket); ok {
		return connection.SendPacket(NewGatePacket(gatePacket.PlayerId(), command, message))
	} else {
		return connection.Send(command, message)
	}
}