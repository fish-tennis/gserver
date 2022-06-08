package misc

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/gameplayer"
)

type GameServerInitArg struct {
	ClientHandler PacketHandlerRegister
	ServerHandler PacketHandlerRegister
	PlayerMgr gameplayer.PlayerMgr
}