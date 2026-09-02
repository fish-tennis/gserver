package network

import (
	"encoding/binary"
	"errors"
	"log/slog"

	. "github.com/fish-tennis/gnet"
	"google.golang.org/protobuf/proto"
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
	// WsClientCodec 自定义的 Decode 会覆盖 gnet SimpleProtoCodec.Decode,
	// 必须同样处理可选的 rpcCallId/errorCode 字段,否则带 RpcCall 标记位的包会被
	// 误把 rpcCallId 的 4 字节当作 message 去解析,导致 WebSocket 链路的 rpcCallId 透传失败。
	// 字段顺序统一为 rpcCallId 在前、errorCode 在后,与 gnet ProtoCodec(TCP链路)一致。
	bodyData := data[SimplePacketHeaderSize:]
	rpcCallId := uint32(0)
	if packetHeader.HasFlag(RpcCall) {
		if len(bodyData) < 4 {
			return nil, errors.New("ws rpcCallId decode err")
		}
		rpcCallId = binary.LittleEndian.Uint32(bodyData[:4])
		bodyData = bodyData[4:]
	}
	errorCode := uint32(0)
	if packetHeader.HasFlag(ErrorCode) {
		if len(bodyData) < 4 {
			return nil, errors.New("ws errorCode decode err")
		}
		errorCode = binary.LittleEndian.Uint32(bodyData[:4])
		bodyData = bodyData[4:]
	}
	if protoMessageCreator, ok := this.MessageCreatorMap[PacketCommand(command)]; ok {
		if protoMessageCreator != nil {
			newProtoMessage := protoMessageCreator()
			err = proto.Unmarshal(bodyData, newProtoMessage)
			if err != nil {
				slog.Error("WsClientCodec.DecodePacket: proto decode error", "error", err, "cmd", command)
				return nil, err
			}
			return NewProtoPacket(PacketCommand(command), newProtoMessage).WithRpc(rpcCallId).SetErrorCode(errorCode), nil
		} else {
			// 支持只注册了消息号,没注册proto结构体的用法
			// support Register(command, nil), return the direct stream data to application layer
			return NewProtoPacketWithData(PacketCommand(command), bodyData).WithRpc(rpcCallId).SetErrorCode(errorCode), nil
		}
	}
	// 其他消息,gate直接转发,附加上playerId
	if clientData, ok := connection.GetTag().(*ClientData); ok {
		return NewGatePacketWithData(clientData.GetPlayerId(), PacketCommand(command), bodyData).WithRpc(rpcCallId).SetErrorCode(errorCode), nil
	}
	// 未绑定连接的业务消息:不能断开连接——重连握手期间(PlayerReconnectGameRes
	// 还未返回前)客户端发的业务消息会被误杀,触发"重连→被踢→再重连"死循环。
	// 注意:gnet 的 readLoop 对 Decode 返回 (nil,nil) 同样会断开连接,
	// 因此"丢弃"只能以放行合法包的形式实现。
	// 这里放行为 ProtoPacket,交给 handler 层 routeToGameServer 的未绑定分支,
	// 它会回 SessionNotBound 错误响应并保持连接
	slog.Warn("WsClientCodec.DecodePacket: unbound client message, route to handler",
		"connId", connection.GetConnectionId(), "command", command)
	return NewProtoPacketWithData(PacketCommand(command), bodyData).WithRpc(rpcCallId).SetErrorCode(errorCode), nil
}
