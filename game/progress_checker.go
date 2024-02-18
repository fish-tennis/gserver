package game

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

func RegisterProgressCheckers() *internal.ProgressMgr {
	progressMgr := internal.NewProgressMgr()
	progressMgr.RegisterWithInit(int32(pb.ProgressType_ProgressType_PlayerLevelup), &pb.EventPlayerLevelup{},
		func(event interface{}, progressCfg *internal.ProgressCfg) int32 {
			// 升级条件,玩家的等级就是进度值
			return event.(*pb.EventPlayerLevelup).GetLevel()
		},
		func(arg interface{}, progressCfg *internal.ProgressCfg) int32 {
			// 初始进度是玩家当时的等级
			return arg.(*Player).GetBaseInfo().Data.GetLevel()
		})

	progressMgr.RegisterDefault(int32(pb.ProgressType_ProgressType_Fight), &pb.EventFight{})

	progressMgr.RegisterWithInit(int32(pb.ProgressType_ProgressType_PlayerPropertyInc), &pb.EventPlayerPropertyInc{},
		func(event interface{}, progressCfg *internal.ProgressCfg) int32 {
			propertyName := progressCfg.GetPropertyString("PropertyName")
			eventPropertyInc := event.(*pb.EventPlayerPropertyInc)
			if propertyName != "" && eventPropertyInc.GetPropertyName() == propertyName {
				logger.Debug("PlayerPropertyInc name:%v value:%v", propertyName, eventPropertyInc.GetPropertyValue())
				return eventPropertyInc.GetPropertyValue()
			}
			return 0
		},
		func(arg interface{}, progressCfg *internal.ProgressCfg) int32 {
			// 当前属性值
			propertyName := progressCfg.GetPropertyString("PropertyName")
			logger.Debug("PlayerPropertyIncInit name:%v value:%v", propertyName, arg.(*Player).GetPropertyInt32(propertyName))
			return arg.(*Player).GetPropertyInt32(propertyName)
		})
	return progressMgr
}
