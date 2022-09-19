package game

import (
	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
)

// 根据proto的命名规则和玩家组件里消息回调的格式,通过反射自动生成消息的注册
// 类似Java的注解功能
func AutoRegisterPlayerComponentProto(packetHandlerRegister PacketHandlerRegister) {
	tmpPlayer := CreateTempPlayer(0, 0)
	gentity.AutoRegisterComponentHandler(tmpPlayer, packetHandlerRegister,
		"On", "Handle", "gserver")
}