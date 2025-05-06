package internal

import (
	"github.com/fish-tennis/gserver/pb"
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
