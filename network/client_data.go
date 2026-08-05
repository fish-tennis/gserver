package network

import (
	"sync/atomic"

	. "github.com/fish-tennis/gnet"
)

// 客户端绑定数据
// ConnId/AccountId 在创建后不可变(immutable),无需同步
// PlayerId 在 onPlayerEntryGameRes 中异步写入,在客户端连接收包协程中读取,用 atomic 保证可见性
// GameServerId 在 onGateRouteClientPacketError 中被清零(写锁内),在 routeToGameServer 中无锁读取,
// 因此也必须用 atomic,否则跨协程读写构成数据竞争
// connection 在创建 ClientData 时(锁内)设置,保存连接引用避免每次转发都通过 connId 查 listener,
// 所有写入都在 clientsMutex 锁临界区内,读取在 RLock 内,无需额外同步
type ClientData struct {
	ConnId       uint32
	AccountId    int64
	gameServerId atomic.Int32
	playerId     atomic.Int64
	connection   Connection
}

func (c *ClientData) GetGameServerId() int32 {
	return c.gameServerId.Load()
}

func (c *ClientData) SetGameServerId(gameServerId int32) {
	c.gameServerId.Store(gameServerId)
}

func (c *ClientData) GetPlayerId() int64 {
	return c.playerId.Load()
}

func (c *ClientData) SetPlayerId(playerId int64) {
	c.playerId.Store(playerId)
}

// GetConnection 返回绑定的客户端连接引用
// 调用方需在持有 clientsMutex(RLock) 时调用,确保与重绑操作串行
func (c *ClientData) GetConnection() Connection {
	return c.connection
}

// SetConnection 设置绑定的客户端连接,需在 clientsMutex 锁临界区内调用
func (c *ClientData) SetConnection(conn Connection) {
	c.connection = conn
}
