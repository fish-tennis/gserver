package social

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/gameplayer"
	. "github.com/fish-tennis/gserver/internal"
)

// 服务器初始化
func OnServerInit(clientHandler PacketHandlerRegister, serverHandler PacketHandlerRegister, playerMgr gameplayer.PlayerMgr) {
	// 注册客户端的消息回调
	GuildClientHandlerRegister(clientHandler, playerMgr)
	// 注册服务器之间的消息回调
	GuildServerHandlerRegister(serverHandler, playerMgr)
	// 服务器列表更新回调
	GetServerList().AddListUpdateHook(onServerListUpdate)
	// 服务器非正常关闭可能导致分布式锁没能释放(如crash),所以服务器启动时,进行自动修复
	deleteGuildServerLock()
}