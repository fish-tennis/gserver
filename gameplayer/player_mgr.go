package gameplayer

type PlayerMgr interface {
	GetPlayer(playerId int64) *Player
}