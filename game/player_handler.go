package game

import . "github.com/fish-tennis/gnet"

// 玩家的普通回调
var _playerHandler = make(map[PacketCommand]func(player *Player, packet Packet))

func RegisterPlayerHandler(cmd PacketCommand, handler func(player *Player, packet Packet)) {
	_playerHandler[cmd] = handler
}