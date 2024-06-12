package game

import (
	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/internal"
)

// 玩家组件接口注册
var (
	_playerComponentHandlerRegister = gentity.NewComponentHandlerRegister()
	// 玩家的普通回调接口注册
	_playerHandlerRegister = make(map[PacketCommand]func(player *Player, packet Packet))
)

// 根据proto的命名规则和玩家组件里消息回调的格式,通过反射自动生成消息的注册
// 类似Java的注解功能
func AutoRegisterPlayerComponentProto(packetHandlerRegister PacketHandlerRegister) {
	tmpPlayer := CreateTempPlayer(0, 0)
	_playerComponentHandlerRegister.AutoRegisterComponentHandlerWithClient(tmpPlayer, packetHandlerRegister,
		internal.ClientHandlerMethodNamePrefix, internal.HandlerMethodNamePrefix, internal.ProtoPackageName)
}

// 注册普通接口
func RegisterPlayerHandler(cmd PacketCommand, handler func(player *Player, packet Packet)) {
	_playerHandlerRegister[cmd] = handler
}
