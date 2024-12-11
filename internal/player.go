package internal

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gnet"
)

type IPlayer interface {
	gentity.Entity

	// 玩家名
	GetName() string

	// 账号id
	GetAccountId() int64

	// 区服id
	GetRegionId() int32

	SendPacket(packet gnet.Packet, opts ...gnet.SendOption) bool
}

type PlayerMgr interface {
	GetPlayer(playerId int64) IPlayer
	AddPlayer(player IPlayer)
	RemovePlayer(player IPlayer)
}

// 加载完数据后的回调接口
type DataLoader interface {
	OnDataLoad()
}
