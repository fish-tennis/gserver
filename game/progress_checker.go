package game

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

// 注册进度接口
func RegisterProgressCheckers() *internal.ProgressMgr {
	progressMgr := internal.NewProgressMgr()

	progressMgr.RegisterDefault(int32(pb.ProgressType_ProgressType_Event),
		&pb.EventFight{},
	)

	progressMgr.RegisterWithInit(int32(pb.ProgressType_ProgressType_PlayerPropertyInc), &pb.EventPlayerPropertyInc{},
		onPlayerPropertyIncUpdate, onPlayerPropertyIncInit)
	return progressMgr
}

// 玩家属性值对应的进度初始化
func onPlayerPropertyIncInit(arg interface{}, progressCfg *pb.ProgressCfg) int32 {
	// 当前属性值
	propertyName := progressCfg.Properties["PropertyName"]
	logger.Debug("PlayerPropertyIncInit name:%v value:%v", propertyName, arg.(*Player).GetPropertyInt32(propertyName))
	return arg.(*Player).GetPropertyInt32(propertyName)
}

// 玩家属性值变化对应的进度更新
func onPlayerPropertyIncUpdate(event interface{}, progressCfg *pb.ProgressCfg) int32 {
	propertyName := progressCfg.Properties["PropertyName"]
	eventPropertyInc := event.(*pb.EventPlayerPropertyInc)
	if propertyName != "" && eventPropertyInc.GetPropertyName() == propertyName {
		logger.Debug("PlayerPropertyInc name:%v value:%v", propertyName, eventPropertyInc.GetPropertyValue())
		return eventPropertyInc.GetPropertyValue()
	}
	return 0
}
