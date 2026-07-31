package network

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/logger"
	"google.golang.org/protobuf/proto"
	"sync/atomic"
)

// 客户端绑定数据
// GameServerId/ConnId/AccountId 在创建后不可变(immutable),无需同步
// PlayerId 在 onPlayerEntryGameRes 中异步写入,在客户端连接收包协程中读取,
// 因此用 atomic 保证可见性和原子性,避免散落在各处的锁或无锁读
type ClientData struct {
	ConnId       uint32
	AccountId    int64
	GameServerId int32
	playerId     atomic.Int64
}

func (c *ClientData) GetPlayerId() int64 {
	return c.playerId.Load()
}

func (c *ClientData) SetPlayerId(playerId int64) {
	c.playerId.Store(playerId)
}

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
	if protoMessageCreator, ok := this.MessageCreatorMap[PacketCommand(command)]; ok {
		if protoMessageCreator != nil {
			newProtoMessage := protoMessageCreator()
			err = proto.Unmarshal(data[SimplePacketHeaderSize:], newProtoMessage)
			if err != nil {
				logger.Error("proto decode err:%v cmd:%v name:%v", err, command, proto.MessageName(newProtoMessage))
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
	if clientData, ok := connection.GetTag().(*ClientData); ok {
		return NewGatePacketWithData(clientData.GetPlayerId(), PacketCommand(command), data[SimplePacketHeaderSize:]), nil
	}
	logger.Error("unSupport command:%v", command)
	return nil, ErrNotSupport
}
