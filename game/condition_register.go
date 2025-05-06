package game

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
)

// 注册条件检查接口
func init() {
	internal.RegisterConditionChecker(int32(pb.ConditionType_ConditionType_PlayerPropertyCompare),
		checkPlayerPropertyCompare)

	internal.RegisterConditionChecker(int32(pb.ConditionType_ConditionType_ActivityPropertyCompare),
		checkActivityPropertyCompare)
}

// 条件参数数值比较
func compareConditionArg(conditionCfg *pb.ConditionCfg, compareValue int32) bool {
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
func checkPlayerPropertyCompare(arg any, conditionCfg *pb.ConditionCfg) bool {
	player := parsePlayer(arg)
	if player == nil {
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
func checkActivityPropertyCompare(arg any, conditionCfg *pb.ConditionCfg) bool {
	activity := parseActivity(arg)
	if activity == nil {
		slog.Error("checkActivityPropertyCompareErr", "arg", arg)
		return false
	}
	propertyName := conditionCfg.GetKey()
	if propertyName == "" {
		slog.Error("checkActivityPropertyCompareErr", "conditionCfg", conditionCfg)
		return false
	}
	propertyValue := activity.GetPropertyInt32(propertyName)
	return compareConditionArg(conditionCfg, propertyValue)
}
