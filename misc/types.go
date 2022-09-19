package misc

import (
	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
)

type GameServerInitArg struct {
	ClientHandler PacketHandlerRegister
	ServerHandler PacketHandlerRegister
	PlayerMgr gentity.PlayerMgr
}