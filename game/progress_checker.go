package game

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

// 注册进度接口
func RegisterProgressCheckers() *internal.ProgressMgr {
	progressMgr := internal.NewProgressMgr()
	progressMgr.RegisterWithInit(int32(pb.ProgressType_ProgressType_PlayerLevelup), &pb.EventPlayerLevelup{},
		onPlayerLevelupUpdate, onPlayerLevelupInit)

	progressMgr.RegisterDefault(int32(pb.ProgressType_ProgressType_Fight), &pb.EventFight{})

	progressMgr.RegisterWithInit(int32(pb.ProgressType_ProgressType_PlayerPropertyInc), &pb.EventPlayerPropertyInc{},
		onPlayerPropertyIncUpdate, onPlayerPropertyIncInit)
	return progressMgr
}

// 玩家等级对应的进度初始化
func onPlayerLevelupInit(arg interface{}, progressCfg *internal.ProgressCfg) int32 {
	// 初始进度是玩家当时的等级
	return arg.(*Player).GetBaseInfo().Data.GetLevel()
}

// 玩家升级对应的进度更新
func onPlayerLevelupUpdate(event interface{}, progressCfg *internal.ProgressCfg) int32 {
	// 升级条件,玩家的等级就是进度值
	return event.(*pb.EventPlayerLevelup).GetLevel()
}

// 玩家属性值对应的进度初始化
func onPlayerPropertyIncInit(arg interface{}, progressCfg *internal.ProgressCfg) int32 {
	// 当前属性值
	propertyName := progressCfg.GetPropertyString("PropertyName")
	logger.Debug("PlayerPropertyIncInit name:%v value:%v", propertyName, arg.(*Player).GetPropertyInt32(propertyName))
	return arg.(*Player).GetPropertyInt32(propertyName)
}

// 玩家属性值变化对应的进度更新
func onPlayerPropertyIncUpdate(event interface{}, progressCfg *internal.ProgressCfg) int32 {
	propertyName := progressCfg.GetPropertyString("PropertyName")
	eventPropertyInc := event.(*pb.EventPlayerPropertyInc)
	if propertyName != "" && eventPropertyInc.GetPropertyName() == propertyName {
		logger.Debug("PlayerPropertyInc name:%v value:%v", propertyName, eventPropertyInc.GetPropertyValue())
		return eventPropertyInc.GetPropertyValue()
	}
	return 0
}
