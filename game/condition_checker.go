package game

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
)

// 注册条件接口
func RegisterConditionCheckers() *internal.ConditionMgr {
	conditionMgr := internal.NewConditionMgr()

	conditionMgr.Register(int32(pb.ConditionType_ConditionType_PlayerPropertyCompare),
		checkPlayerPropertyCompare)

	conditionMgr.Register(int32(pb.ConditionType_ConditionType_ActivityPropertyCompare),
		checkActivityPropertyCompare)

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
		// TODO: 可扩展关键字: and or not
	}
	slog.Error("compareConditionArgErr", "op", conditionCfg.Op)
	return false
}

// 玩家属性值比较
func checkPlayerPropertyCompare(arg interface{}, conditionCfg *internal.ConditionCfg) bool {
	player, ok := arg.(*Player)
	if !ok {
		slog.Error("checkPlayerPropertyCompareErr", "arg", arg)
		return false
	}
	propertyName := conditionCfg.GetKey()
	if propertyName == "" {
		slog.Error("checkPlayerPropertyCompareErr", "conditionCfg", conditionCfg)
		return false
	}
	propertyValue := player.GetPropertyInt32(propertyName)
	return compareConditionArg(conditionCfg, propertyValue)
}

// 活动属性值比较
func checkActivityPropertyCompare(arg interface{}, conditionCfg *internal.ConditionCfg) bool {
	activityConditionArg, ok := arg.(*ActivityConditionArg)
	if !ok {
		slog.Error("checkActivityPropertyCompareErr", "arg", arg)
		return false
	}
	propertyName := conditionCfg.GetKey()
	if propertyName == "" {
		slog.Error("checkActivityPropertyCompareErr", "conditionCfg", conditionCfg)
		return false
	}
	propertyValue := activityConditionArg.Activity.GetPropertyInt32(propertyName)
	return compareConditionArg(conditionCfg, propertyValue)
}
