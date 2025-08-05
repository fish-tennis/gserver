package network

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/pb"
)

var (
	ClientConnectionConfig = gnet.ConnectionConfig{
		SendPacketCacheCap: 32,
		SendBufferSize:     8 * 1024,  // 8K
		RecvBufferSize:     8 * 1024,  // 8K
		MaxPacketSize:      16 * 1024, // 16K
		RecvTimeout:        20,        // seconds
	}

	WebSocketClientConnectionConfig = gnet.ConnectionConfig{
		SendPacketCacheCap: 32,
		SendBufferSize:     8 * 1024,  // 8K
		RecvBufferSize:     8 * 1024,  // 8K
		MaxPacketSize:      16 * 1024, // 16K
		RecvTimeout:        20,        // seconds
		Path:               "/ws",
	}

	GateConnectionConfig = gnet.ConnectionConfig{
		SendPacketCacheCap: 128,
		SendBufferSize:     512 * 1024,  // 512K
		RecvBufferSize:     512 * 1024,  // 512K
		MaxPacketSize:      1024 * 1024, // 1M
		RecvTimeout:        5,           // second
		HeartBeatInterval:  2,           // second
	}

	ServerConnectionConfig = gnet.ConnectionConfig{
		SendPacketCacheCap: 128,
		SendBufferSize:     512 * 1024,  // 512K
		RecvBufferSize:     512 * 1024,  // 512K
		MaxPacketSize:      1024 * 1024, // 1M
		RecvTimeout:        5,           // second
		HeartBeatInterval:  2,           // second
	}
)

// 非网关服务器监听普通TCP客户端
func ListenClient(listenAddr string, listenerHandler gnet.ListenerHandler, packetRegister func(handler *gnet.DefaultConnectionHandler)) gnet.Listener {
	codec := gnet.NewProtoCodec(nil)
	handler := gnet.NewDefaultConnectionHandler(codec)
	RegisterPacketHandler(handler, new(pb.HeartBeatReq), onHeartBeatReq)

	packetRegister(handler)
	listenerConfig := &gnet.ListenerConfig{
		AcceptConfig: ClientConnectionConfig,
	}
	listenerConfig.AcceptConfig.Codec = codec
	listenerConfig.AcceptConfig.Handler = handler
	listenerConfig.ListenerHandler = listenerHandler
	return gnet.GetNetMgr().NewListener(gentity.GetApplication().GetContext(), listenAddr, listenerConfig)
}

// 网关服务器监听普通TCP客户端
func ListenGateClient(listenAddr string, listenerHandler gnet.ListenerHandler, packetRegister func(handler *gnet.DefaultConnectionHandler)) gnet.Listener {
	codec := NewClientCodec()
	handler := gnet.NewDefaultConnectionHandler(codec)
	RegisterPacketHandler(handler, new(pb.HeartBeatReq), onHeartBeatReq)

	packetRegister(handler)
	listenerConfig := &gnet.ListenerConfig{
		AcceptConfig: ClientConnectionConfig,
	}
	listenerConfig.AcceptConfig.Codec = codec
	listenerConfig.AcceptConfig.Handler = handler
	listenerConfig.ListenerHandler = listenerHandler
	return gnet.GetNetMgr().NewListener(gentity.GetApplication().GetContext(), listenAddr, listenerConfig)
}

// 网关服务器监听WebSocket客户端
func ListenWebSocketClient(listenAddr string, listenerHandler gnet.ListenerHandler, packetRegister func(handler *gnet.DefaultConnectionHandler)) gnet.Listener {
	codec := NewWsClientCodec()
	handler := gnet.NewDefaultConnectionHandler(codec)
	RegisterPacketHandler(handler, new(pb.HeartBeatReq), onHeartBeatReq)

	packetRegister(handler)
	listenerConfig := &gnet.ListenerConfig{
		AcceptConfig: WebSocketClientConnectionConfig,
	}
	listenerConfig.AcceptConfig.Codec = codec
	listenerConfig.AcceptConfig.Handler = handler
	listenerConfig.ListenerHandler = listenerHandler
	listenerConfig.Path = WebSocketClientConnectionConfig.Path
	return gnet.GetNetMgr().NewWsListener(gentity.GetApplication().GetContext(), listenAddr, listenerConfig)
}

// 其他服务器监听网关
func ListenGate(listenAddr string, packetRegister func(handler *gnet.DefaultConnectionHandler)) gnet.Listener {
	codec := NewGateCodec(nil)
	handler := gnet.NewDefaultConnectionHandler(codec)
	RegisterPacketHandler(handler, new(pb.HeartBeatReq), onHeartBeatReq)

	packetRegister(handler)
	listenerConfig := &gnet.ListenerConfig{
		AcceptConfig: GateConnectionConfig,
	}
	listenerConfig.AcceptConfig.Codec = codec
	listenerConfig.AcceptConfig.Handler = handler
	return gnet.GetNetMgr().NewListener(gentity.GetApplication().GetContext(), listenAddr, listenerConfig)
}

// 默认的心跳回复
func onHeartBeatReq(connection gnet.Connection, packet gnet.Packet) {
	req := packet.Message().(*pb.HeartBeatReq)
	// 兼容普通消息和GatePacket
	SendPacketAdapt(connection, packet, &pb.HeartBeatRes{
		RequestTimestamp:  req.GetTimestamp(),
		ResponseTimestamp: util.GetCurrentMS(),
	})
}
