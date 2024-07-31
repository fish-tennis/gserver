package game

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
)

var (
	// GlobalEntity组件注册表
	_globalEntityComponentRegister = gentity.ComponentRegister[*GlobalEntity]{}

	// GlobalEntity消息回调接口注册
	_globalEntityPacketHandlerMgr = internal.NewPacketHandlerMgr()
)

// 注册GlobalEntity组件构造信息
func RegisterGlobalEntityComponentCtor(componentName string, ctorOrder int, ctor func(globalEntity *GlobalEntity, loadData *pb.GlobalEntityData) gentity.Component) {
	_globalEntityComponentRegister.Register(componentName, ctorOrder, func(globalEntity *GlobalEntity, arg any) gentity.Component {
		return ctor(globalEntity, arg.(*pb.GlobalEntityData))
	})
}
