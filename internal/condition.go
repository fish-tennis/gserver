package internal

import (
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
)

var (
	// singleton
	_conditionCheckers = make(map[int32]ConditionCheckFunc) // key:conditionType
	_valueCompareFuncs = make(map[string]ValueCompareFunc)  // key: op
)

// 条件检查接口
type ConditionCheckFunc func(obj any, conditionCfg *pb.ConditionCfg) bool

// 值比较接口
type ValueCompareFunc func(obj any, compareValue int32, valueCompareCfg *pb.ValueCompareCfg) bool

func GetConditionChecker(conditionType int32) ConditionCheckFunc {
	return _conditionCheckers[conditionType]
}

// 注册条件检查接口
func RegisterConditionChecker(conditionType int32, checker ConditionCheckFunc) {
	_conditionCheckers[conditionType] = checker
}

// 注册值比较接口
func RegisterValueCompareFunc(op string, valueCompareFunc ValueCompareFunc) {
	_valueCompareFuncs[op] = valueCompareFunc
}

func CheckCondition(obj any, conditionCfg *pb.ConditionCfg) bool {
	checker, ok := _conditionCheckers[conditionCfg.Type]
	if !ok {
		return false
	}
	return checker(obj, conditionCfg)
}

func CheckConditions(obj any, conditions []*pb.ConditionCfg) bool {
	for _, conditionCfg := range conditions {
		checker, ok := _conditionCheckers[conditionCfg.Type]
		if !ok {
			return false
		}
		if !checker(obj, conditionCfg) {
			return false
		}
	}
	return true
}

func CompareOpValue(obj any, compareValue int32, valueCompareCfg *pb.ValueCompareCfg) bool {
	if len(valueCompareCfg.Values) == 0 {
		slog.Error("CompareOpValueErr", "compareValue", compareValue, "valueCompareCfg", valueCompareCfg)
		return false
	}
	switch valueCompareCfg.Op {
	case "=":
		// 满足其中一个即可
		for _, value := range valueCompareCfg.Values {
			if compareValue == value {
				return true
			}
		}
		return false
	case ">":
		return compareValue > valueCompareCfg.Values[0]
	case ">=":
		return compareValue >= valueCompareCfg.Values[0]
	case "<":
		return compareValue < valueCompareCfg.Values[0]
	case "<=":
		return compareValue <= valueCompareCfg.Values[0]
	case "!=":
		for _, arg := range valueCompareCfg.Values {
			if compareValue == arg {
				return false
			}
		}
		return true
	case "[]":
		// 可以配置多个区间,如Args:[1,3,8,15]表示[1,3] [8,15]
		for i := 0; i < len(valueCompareCfg.Values); i += 2 {
			if i+1 < len(valueCompareCfg.Values) {
				if compareValue >= valueCompareCfg.Values[i] && compareValue <= valueCompareCfg.Values[i+1] {
					return true
				}
			}
		}
		return false
	case "![]":
		// 可以配置多个区间,如Args:[1,3,8,15]表示[1,3] [8,15]
		for i := 0; i < len(valueCompareCfg.Values); i += 2 {
			if i+1 < len(valueCompareCfg.Values) {
				if compareValue >= valueCompareCfg.Values[i] && compareValue <= valueCompareCfg.Values[i+1] {
					return false
				}
			}
		}
		return true
	default:
		// 扩展: 可以提供注册新的op的接口
		if fn, ok := _valueCompareFuncs[valueCompareCfg.Op]; ok {
			return fn(obj, compareValue, valueCompareCfg)
		}
		slog.Error("CompareOpValueErr", "compareValue", compareValue, "valueCompareCfg", valueCompareCfg)
		return false
	}
}

// PropertyInt32的实现类的属性值比较接口
func DefaultPropertyInt32Checker(obj any, conditionCfg *pb.ConditionCfg) bool {
	if propertyGetter, ok := obj.(PropertyInt32); ok {
		propertyName := conditionCfg.GetKey()
		if propertyName == "" {
			slog.Error("DefaultPropertyInt32CheckerErr", "obj", obj, "conditionCfg", conditionCfg)
			return false
		}
		propertyValue := propertyGetter.GetPropertyInt32(propertyName)
		return CompareOpValue(obj, propertyValue, &pb.ValueCompareCfg{
			Op:     conditionCfg.GetOp(),
			Values: conditionCfg.GetValues(),
		})
	}
	slog.Error("DefaultPropertyInt32CheckerErr", "obj", obj, "conditionCfg", conditionCfg)
	return false
}
