package internal

import (
	"github.com/fish-tennis/gserver/pb"
)

// 条件检查接口
type ConditionCheckFunc func(arg any, conditionCfg *pb.ConditionCfg) bool

// 条件相关接口管理
type ConditionMgr struct {
	conditionCheckers map[int32]ConditionCheckFunc
}

func NewConditionMgr() *ConditionMgr {
	return &ConditionMgr{
		conditionCheckers: make(map[int32]ConditionCheckFunc),
	}
}

func (m *ConditionMgr) GetConditionChecker(conditionType int32) ConditionCheckFunc {
	return m.conditionCheckers[conditionType]
}

// 注册条件检查接口
func (m *ConditionMgr) Register(conditionType int32, checker ConditionCheckFunc) {
	m.conditionCheckers[conditionType] = checker
}

func (m *ConditionMgr) CheckConditions(arg any, conditions []*pb.ConditionCfg) bool {
	for _, conditionCfg := range conditions {
		checker, ok := m.conditionCheckers[conditionCfg.Type]
		if !ok {
			return false
		}
		if !checker(arg, conditionCfg) {
			return false
		}
	}
	return true
}
