package social

import (
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/misc"
)

type Hook struct {
}

// 服务器初始化回调
func (h *Hook) OnServerInit(initArg interface{}) {
	arg := initArg.(*misc.GameServerInitArg)
	initGuildMgr()
	// 注册客户端的消息回调
	GuildClientHandlerRegister(arg.ClientHandler, arg.PlayerMgr)
	// 注册服务器之间的消息回调
	GuildServerHandlerRegister(arg.ServerHandler, arg.PlayerMgr)
	// 服务器列表更新回调
	GetServerList().AddListUpdateHook(onServerListUpdate)
	// 服务器非正常关闭可能导致分布式锁没能释放(如crash),所以服务器启动时,进行自动修复
	_guildMgr.DeleteDistributeLocks()
}

// 服务器关闭回调
func (h *Hook) OnServerExit() {
	_guildMgr.StopAll()
}
