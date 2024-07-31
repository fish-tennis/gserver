package game

import (
	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/internal"
)

// 玩家接口注册
var (
	_playerPacketHandlerMgr = internal.NewPacketHandlerMgr()
	// 玩家的普通回调接口注册
	_playerHandlerRegister = make(map[PacketCommand]func(player *Player, packet Packet))
	// 玩家的事件响应接口注册
	_playerEventHandlerMgr = gentity.NewEventHandlerMgr()
)

// 根据proto的命名规则和玩家组件里消息回调的格式,通过反射自动生成消息的注册
// 类似Java的注解功能
func AutoRegisterPlayerPacketHandler(packetHandlerRegister PacketHandlerRegister) {
	tmpPlayer := CreateTempPlayer(0, 0)
	_playerPacketHandlerMgr.AutoRegisterWithClient(tmpPlayer, packetHandlerRegister,
		internal.ClientHandlerMethodNamePrefix, internal.HandlerMethodNamePrefix, internal.ProtoPackageName)
}

// 注册普通接口
func RegisterPlayerHandler(cmd PacketCommand, handler func(player *Player, packet Packet)) {
	_playerHandlerRegister[cmd] = handler
}

func InitPlayerStructAndHandler() {
	tmpPlayer := CreateTempPlayer(0, 0)
	gentity.GetEntitySaveableStruct(tmpPlayer)
	_playerEventHandlerMgr.AutoRegister(tmpPlayer, internal.EventHandlerMethodNamePrefix)
}
