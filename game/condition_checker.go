package game

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

func RegisterConditionCheckers() *internal.ConditionMgr {
	conditionMgr := internal.NewConditionMgr()
	conditionMgr.RegisterWithInit(int32(pb.ConditionType_ConditionType_PlayerLevelup), &pb.EventPlayerLevelup{},
		func(event interface{}, conditionCfg *internal.ConditionCfg) int32 {
			// 升级条件,玩家的等级就是进度值
			return event.(*pb.EventPlayerLevelup).GetLevel()
		},
		func(arg interface{}, conditionCfg *internal.ConditionCfg) int32 {
			// 初始进度是玩家当时的等级
			return arg.(*Player).GetBaseInfo().Data.GetLevel()
		})

	conditionMgr.RegisterDefault(int32(pb.ConditionType_ConditionType_Fight), &pb.EventFight{})

	conditionMgr.RegisterWithInit(int32(pb.ConditionType_ConditionType_PlayerPropertyInc), &pb.EventPlayerPropertyInc{},
		func(event interface{}, conditionCfg *internal.ConditionCfg) int32 {
			propertyName := conditionCfg.GetEventArgString("PropertyName")
			eventPropertyInc := event.(*pb.EventPlayerPropertyInc)
			if propertyName != "" && eventPropertyInc.GetPropertyName() == propertyName {
				logger.Debug("PlayerPropertyInc name:%v value:%v", propertyName, eventPropertyInc.GetPropertyValue())
				return eventPropertyInc.GetPropertyValue()
			}
			return 0
		},
		func(arg interface{}, conditionCfg *internal.ConditionCfg) int32 {
			// 当前属性值
			propertyName := conditionCfg.GetEventArgString("PropertyName")
			logger.Debug("PlayerPropertyIncInit name:%v value:%v", propertyName, arg.(*Player).GetPropertyInt32(propertyName))
			return arg.(*Player).GetPropertyInt32(propertyName)
		})
	return conditionMgr
}
