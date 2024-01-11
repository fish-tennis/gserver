package gate

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/logger"
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
	if clientData,ok := connection.GetTag().(*ClientData); ok {
		connection.SetTag(nil)
		if clientData.PlayerId > 0 {
			delete(_gateServer.clients, clientData.PlayerId)
		}
		logger.Debug("ClientDisconnect connId:%v playerId:%v", connection.GetConnectionId(), clientData.PlayerId)
	}
}
