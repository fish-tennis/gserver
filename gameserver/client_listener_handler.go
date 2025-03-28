package gameserver

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/game"
	"log/slog"
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
	if playerId, ok := connection.GetTag().(int64); ok {
		player := game.GetPlayer(playerId)
		if player == nil {
			return
		}
		//if atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&player.connection)), unsafe.Pointer(&connection), nil) {
		slog.Info("OnConnectionDisconnect", "playerId", playerId, "connId", connection.GetConnectionId())
		player.OnDisconnect(connection)
	}
}
