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
		if len(conditionCfg.Args) == 0 {
			slog.Error("compareConditionArgErr", "conditionCfg", conditionCfg)
			return false
		}
		return compareValue == conditionCfg.Args[0]
	case ">":
		if len(conditionCfg.Args) == 0 {
			slog.Error("compareConditionArgErr", "conditionCfg", conditionCfg)
			return false
		}
		return compareValue > conditionCfg.Args[0]
	case ">=":
		if len(conditionCfg.Args) == 0 {
			slog.Error("compareConditionArgErr", "conditionCfg", conditionCfg)
			return false
		}
		return compareValue >= conditionCfg.Args[0]
	case "<":
		if len(conditionCfg.Args) == 0 {
			slog.Error("compareConditionArgErr", "conditionCfg", conditionCfg)
			return false
		}
		return compareValue < conditionCfg.Args[0]
	case "<=":
		if len(conditionCfg.Args) == 0 {
			slog.Error("compareConditionArgErr", "conditionCfg", conditionCfg)
			return false
		}
		return compareValue <= conditionCfg.Args[0]
	case "!=":
		if len(conditionCfg.Args) == 0 {
			slog.Error("compareConditionArgErr", "conditionCfg", conditionCfg)
			return false
		}
		return compareValue != conditionCfg.Args[0]
	case "in":
		// 满足其中一个即可
		for _, arg := range conditionCfg.Args {
			if compareValue == arg {
				return true
			}
		}
		return false
	case "nin": // not in
		for _, arg := range conditionCfg.Args {
			if compareValue == arg {
				return false
			}
		}
		return true
	case "[]":
		// 可以配置多个区间,如Args:[1,3,8,15]表示[1,3] [8,15]
		for i := 0; i < len(conditionCfg.Args); i += 2 {
			if i+1 < len(conditionCfg.Args) {
				if compareValue >= conditionCfg.Args[i] && compareValue <= conditionCfg.Args[i+1] {
					return true
				}
			}
		}
		return false
	case "![]":
		// 可以配置多个区间,如Args:[1,3,8,15]表示[1,3] [8,15]
		for i := 0; i < len(conditionCfg.Args); i += 2 {
			if i+1 < len(conditionCfg.Args) {
				if compareValue >= conditionCfg.Args[i] && compareValue <= conditionCfg.Args[i+1] {
					return false
				}
			}
		}
		return true
	}
	// 扩展设想: 如果有更复杂的需求,可以把OP作为函数名,来实现类似脚本的效果
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
