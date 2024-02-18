package internal

import (
	"github.com/fish-tennis/gserver/pb"
)

// 条件配置数据
type ConditionCfg struct {
	pb.BaseConditionCfg
	BaseProperties // 动态属性
}

// 条件检查接口
type ConditionCheckFunc func(arg interface{}, conditionCfg *ConditionCfg) bool

// 条件相关接口管理
type ConditionMgr struct {
	conditionCheckers map[int32]ConditionCheckFunc
}

func NewConditionMgr() *ConditionMgr {
	return &ConditionMgr{
		conditionCheckers: make(map[int32]ConditionCheckFunc),
	}
}

func (this *ConditionMgr) GetConditionChecker(conditionType int32) ConditionCheckFunc {
	return this.conditionCheckers[conditionType]
}

// 注册条件检查接口
func (this *ConditionMgr) Register(conditionType int32, checker ConditionCheckFunc) {
	this.conditionCheckers[conditionType] = checker
}

func (this *ConditionMgr) CheckConditions(arg interface{}, conditions []*ConditionCfg) bool {
	for _,conditionCfg := range conditions {
		checker,ok := this.conditionCheckers[conditionCfg.Type]
		if !ok {
			return false
		}
		if !checker(arg, conditionCfg) {
			return false
		}
	}
	return true
}