package gameplayer

var _playerMgr PlayerMgr

type PlayerMgr interface {
	GetPlayer(playerId int64) *Player
}

func SetPlayerMgr(mgr PlayerMgr) {
	_playerMgr = mgr
}

func GetPlayerMgr() PlayerMgr {
	return _playerMgr
}