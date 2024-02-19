package game

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

func RegisterConditionCheckers() *internal.ConditionMgr {
	conditionMgr := internal.NewConditionMgr()

	conditionMgr.Register(int32(pb.ConditionType_ConditionType_PlayerPropertyCompare), func(arg interface{}, conditionCfg *internal.ConditionCfg) bool {
		player,ok := arg.(*Player)
		if !ok {
			logger.Error("arg not Player: %v", arg)
			return false
		}
		propertyName := conditionCfg.GetPropertyString("PropertyName")
		if propertyName == "" {
			logger.Error("not find PropertyName: %v", conditionCfg)
			return false
		}
		propertyValue := player.GetPropertyInt32(propertyName)
		return compareConditionArg(conditionCfg, propertyValue)
	})

	conditionMgr.Register(int32(pb.ConditionType_ConditionType_ActivityPropertyCompare), func(arg interface{}, conditionCfg *internal.ConditionCfg) bool {
		activityConditionArg,ok := arg.(*ActivityConditionArg)
		if !ok {
			logger.Error("arg not ActivityConditionArg: %v", arg)
			return false
		}
		propertyName := conditionCfg.GetPropertyString("PropertyName")
		if propertyName == "" {
			logger.Error("not find PropertyName: %v", conditionCfg)
			return false
		}
		propertyValue := activityConditionArg.Activity.GetPropertyInt32(propertyName)
		return compareConditionArg(conditionCfg, propertyValue)
	})

	return conditionMgr
}

// 条件参数数值比较
func compareConditionArg(conditionCfg *internal.ConditionCfg, compareValue int32) bool {
	switch conditionCfg.Op {
	case "=":
		return compareValue == conditionCfg.Arg
	case ">":
		return compareValue > conditionCfg.Arg
	case ">=":
		return compareValue >= conditionCfg.Arg
	case "<":
		return compareValue < conditionCfg.Arg
	case "<=":
		return compareValue <= conditionCfg.Arg
	}
	logger.Error("wrong op:%v", conditionCfg.Op)
	return false
}
