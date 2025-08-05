package game

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
)

var (
	_globalEntity *GlobalEntity
)

func GetGlobalEntity() *GlobalEntity {
	return _globalEntity
}

type Hook struct {
}

func (h *Hook) OnRegisterServerHandler(_ any) {
}

// 服务器初始化回调
func (h *Hook) OnApplicationInit(initArg interface{}) {
	InitGlobalEntityStructAndHandler()
	_globalEntity = CreateGlobalEntityFromDb()
	_globalEntity.RunRoutine()
	cmd := network.GetCommandByProto(new(pb.StartupReq))
	_globalEntity.PushMessage(NewProtoPacket(PacketCommand(cmd), &pb.StartupReq{
		Timestamp: GetGlobalEntity().GetTimerEntries().Now().Unix(),
	}))
}

// 服务器关闭回调
func (h *Hook) OnApplicationExit() {
}
