package game

import (
	"github.com/fish-tennis/gentity"
)

var _playerMgr gentity.PlayerMgr

func SetPlayerMgr(mgr gentity.PlayerMgr) {
	_playerMgr = mgr
}

func GetPlayerMgr() gentity.PlayerMgr {
	return _playerMgr
}

func GetPlayer(playerId int64) *Player {
	player := _playerMgr.GetPlayer(playerId)
	if player == nil {
		return nil
	}
	return player.(*Player)
}
