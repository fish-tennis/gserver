package game

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
	if playerId,ok := connection.GetTag().(int64); ok {
		player := _gameServer.GetPlayer(playerId)
		if player == nil {
			return
		}
		//if atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&player.connection)), unsafe.Pointer(&connection), nil) {
		if player.GetConnection() == connection {
			player.SetConnection(nil)
			player.Stop()
		}
		logger.Debug("player %v exit", player.GetId())
	}
}
