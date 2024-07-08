package social

import (
	"github.com/fish-tennis/gnet"
	. "github.com/fish-tennis/gserver/internal"
)

type Hook struct {
}

func (h *Hook) OnRegisterServerHandler(serverHandler gnet.ConnectionHandler) {
	InitGuildStructAndHandler()
	// 注册服务器之间的消息回调
	GuildServerHandlerRegister(serverHandler.(gnet.PacketHandlerRegister))
}

// 服务器初始化回调
func (h *Hook) OnApplicationInit(initArg interface{}) {
	initGuildMgr()
	// 服务器列表更新回调
	GetServerList().AddListUpdateHook(onServerListUpdate)
	// 服务器非正常关闭可能导致分布式锁没能释放(如crash),所以服务器启动时,进行自动修复
	_guildMgr.DeleteDistributeLocks()
}

// 服务器关闭回调
func (h *Hook) OnApplicationExit() {
	_guildMgr.StopAll()
}
