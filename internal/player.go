package internal

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gnet"
	"google.golang.org/protobuf/proto"
)

type IPlayer interface {
	gentity.Entity

	// 玩家名
	GetName() string

	// 账号id
	GetAccountId() int64

	// 区服id
	GetRegionId() int32

	// TODO:移除gnet依赖
	Send(command gnet.PacketCommand, message proto.Message, opts ...gnet.SendOption) bool

	SendPacket(packet gnet.Packet, opts ...gnet.SendOption) bool
}

type PlayerMgr interface {
	GetPlayer(playerId int64) IPlayer
	AddPlayer(player IPlayer)
	RemovePlayer(player IPlayer)
}
