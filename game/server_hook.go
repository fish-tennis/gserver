package game

import (
	"log/slog"

	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
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
	// 注册虚拟时钟偏移变更回调:时间快进后投递tick唤醒GlobalEntity的定时器,
	// 立即收割虚拟时间已到期的任务,而不是等真实剩余时长(见globalEntityTickMessage)
	// 使用TryPushMessage非阻塞:回调运行在订阅协程,实体协程积压时不应阻塞订阅
	cache.RegisterGameTimeChangeCallback(func() {
		if e := GetGlobalEntity(); e != nil {
			if !e.TryPushMessage(&globalEntityTickMessage{}) {
				slog.Warn("kick global entity timer: channel full")
			}
		}
	})
	cmd := network.GetCommandByProto(new(pb.StartupReq))
	_globalEntity.PushMessage(NewProtoPacket(PacketCommand(cmd), &pb.StartupReq{
		Timestamp: GetGlobalEntity().GetTimerEntries().Now().Unix(),
	}))
}

// 服务器关闭回调
func (h *Hook) OnApplicationExit() {
	// 停止 GlobalEntity 协程,确保其 EndFunc(含 SaveDb)在基础设施关闭前完成
	if _globalEntity != nil {
		_globalEntity.Stop()
	}
}
