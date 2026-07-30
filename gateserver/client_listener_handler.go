package gateserver

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
)

// 客户端listener handler
type ClientListerHandler struct {
}

func (this *ClientListerHandler) OnConnectionConnected(listener Listener, acceptedConnection Connection) {
}

// 客户端断开连接
func (this *ClientListerHandler) OnConnectionDisconnect(listener Listener, connection Connection) {
	if connection.GetTag() == nil {
		return
	}
	if clientData, ok := connection.GetTag().(*network.ClientData); ok {
		connection.SetTag(nil)
		if clientData.PlayerId > 0 {
			// 持锁后校验当前映射确实属于本连接的 clientData,避免重连后旧连接误删新连接的映射
			_gateServer.clientsMutex.Lock()
			if current, exists := _gateServer.clients[clientData.PlayerId]; exists && current == clientData {
				delete(_gateServer.clients, clientData.PlayerId)
				_gateServer.clientsMutex.Unlock()
				// 通知GameServer,玩家掉线了
				_gateServer.GetServerList().SendPacket(clientData.GameServerId, network.NewGatePacket(
					clientData.PlayerId, 0, &pb.ClientDisconnect{
						ClientConnId: connection.GetConnectionId(),
					}))
			} else {
				// 映射已被重连替换,不能删除,也不发断线通知
				_gateServer.clientsMutex.Unlock()
			}
		}
		logger.Debug("ClientDisconnect connId:%v playerId:%v", connection.GetConnectionId(), clientData.PlayerId)
	}
}
