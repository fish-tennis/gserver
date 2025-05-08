package internal

import (
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
)

var (
	// singleton
	_conditionCheckers = make(map[int32]ConditionCheckFunc) // key:conditionType
)

// 条件检查接口
type ConditionCheckFunc func(arg any, conditionCfg *pb.ConditionCfg) bool

func GetConditionChecker(conditionType int32) ConditionCheckFunc {
	return _conditionCheckers[conditionType]
}

// 注册条件检查接口
func RegisterConditionChecker(conditionType int32, checker ConditionCheckFunc) {
	_conditionCheckers[conditionType] = checker
}

func CheckCondition(arg any, conditionCfg *pb.ConditionCfg) bool {
	checker, ok := _conditionCheckers[conditionCfg.Type]
	if !ok {
		return false
	}
	return checker(arg, conditionCfg)
}

func CheckConditions(arg any, conditions []*pb.ConditionCfg) bool {
	for _, conditionCfg := range conditions {
		checker, ok := _conditionCheckers[conditionCfg.Type]
		if !ok {
			return false
		}
		if !checker(arg, conditionCfg) {
			return false
		}
	}
	return true
}

func CompareOpValue(compareValue int32, op string, values []int32) bool {
	if len(values) == 0 {
		slog.Error("CompareOpValueErr", "compareValue", compareValue, "op", op)
		return false
	}
	switch op {
	case "=":
		// 满足其中一个即可
		for _, arg := range values {
			if compareValue == arg {
				return true
			}
		}
		return false
	case ">":
		return compareValue > values[0]
	case ">=":
		return compareValue >= values[0]
	case "<":
		return compareValue < values[0]
	case "<=":
		return compareValue <= values[0]
	case "!=":
		for _, arg := range values {
			if compareValue == arg {
				return false
			}
		}
		return true
	case "[]":
		// 可以配置多个区间,如Args:[1,3,8,15]表示[1,3] [8,15]
		for i := 0; i < len(values); i += 2 {
			if i+1 < len(values) {
				if compareValue >= values[i] && compareValue <= values[i+1] {
					return true
				}
			}
		}
		return false
	case "![]":
		// 可以配置多个区间,如Args:[1,3,8,15]表示[1,3] [8,15]
		for i := 0; i < len(values); i += 2 {
			if i+1 < len(values) {
				if compareValue >= values[i] && compareValue <= values[i+1] {
					return false
				}
			}
		}
		return true
	}
	// 扩展设想: 如果有更复杂的需求,可以把OP作为函数名,用反射来实现类似脚本的效果
	slog.Error("CompareOpValueErr", "compareValue", compareValue, "op", op, "values", values)
	return false
}
