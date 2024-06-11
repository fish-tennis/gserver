package social

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/pb"
)

var (
	// 公会组件注册表
	_guildComponentRegister = gentity.ComponentRegister[*Guild]{}
)

// 注册公会组件构造信息
func RegisterGuildComponentCtor(componentName string, ctorOrder int, ctor func(guild *Guild, guildData *pb.GuildLoadData) gentity.Component) {
	_guildComponentRegister.Register(componentName, ctorOrder, func(entity *Guild, arg any) gentity.Component {
		return ctor(entity, arg.(*pb.GuildLoadData))
	})
}
