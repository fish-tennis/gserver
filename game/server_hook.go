package game

import (
	. "github.com/fish-tennis/gnet"
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

// 服务器初始化回调
func (h *Hook) OnApplicationInit(initArg interface{}) {
	InitGlobalEntityStructAndHandler()
	_globalEntity = CreateGlobalEntityFromDb()
	_globalEntity.RunRoutine()
	_globalEntity.PushMessage(NewProtoPacket(PacketCommand(pb.CmdGlobalEntity_Cmd_StartupReq), &pb.StartupReq{
		Timestamp: GetGlobalEntity().GetTimerEntries().Now().Unix(),
	}))
}

// 服务器关闭回调
func (h *Hook) OnApplicationExit() {
}