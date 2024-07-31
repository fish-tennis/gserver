package game

import (
	"github.com/fish-tennis/gserver/internal"
)

var _playerMgr internal.PlayerMgr

func SetPlayerMgr(mgr internal.PlayerMgr) {
	_playerMgr = mgr
}

func GetPlayerMgr() internal.PlayerMgr {
	return _playerMgr
}

func GetPlayer(playerId int64) *Player {
	player := _playerMgr.GetPlayer(playerId)
	if player == nil {
		return nil
	}
	return player.(*Player)
}
