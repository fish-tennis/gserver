package game

import (
	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/internal"
)

// 玩家接口注册
var (
	// 玩家的消息回调接口注册
	_playerPacketHandlerMgr = internal.NewPacketHandlerMgr()
	// 玩家的事件响应接口注册
	_playerEventHandlerMgr = gentity.NewEventHandlerMgr()
)

// 根据proto的命名规则和玩家组件里消息回调的格式,通过反射自动生成消息的注册
// 类似Java的注解功能
func AutoRegisterPlayerPacketHandler(packetHandlerRegister PacketHandlerRegister) {
	tmpPlayer := CreateTempPlayer(0, 0)
	_playerPacketHandlerMgr.AutoRegisterWithClient(tmpPlayer, packetHandlerRegister,
		internal.ClientHandlerMethodNamePrefix, internal.HandlerMethodNamePrefix)
}

func InitPlayerStructAndHandler() {
	tmpPlayer := CreateTempPlayer(0, 0)
	gentity.ParseEntitySaveableStruct(tmpPlayer)
	_playerEventHandlerMgr.AutoRegister(tmpPlayer, internal.EventHandlerMethodNamePrefix)
}
