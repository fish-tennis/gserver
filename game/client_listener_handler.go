package game

import "github.com/fish-tennis/gnet"

// 客户端listener handler
type ClientListerHandler struct {
	
}

func (this *ClientListerHandler) OnConnectionConnected(listener gnet.Listener, acceptedConnection gnet.Connection) {
}

func (this *ClientListerHandler) OnConnectionDisconnect(listener gnet.Listener, connection gnet.Connection) {
	if connection.GetTag() == nil {
		return
	}
	playerId := connection.GetTag().(int64)
	player := gameServer.GetPlayer(playerId)
	if player == nil {
		return
	}
	player.Save()
	gameServer.RemovePlayer(player)
	gnet.LogDebug("player %v exit", player.GetId())
}
