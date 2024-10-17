package cfg

import (
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
)

var (
	_questCfgLoader = Register(func() any {
		return new(QuestCfgMgr)
	}, Mid)
)

// 任务配置数据
type QuestCfg struct {
	pb.BaseQuestCfg
	Conditions     []*ConditionCfg `json:"Conditions"`  // 条件配置
	ProgressCfg    *ProgressCfg    `json:"ProgressCfg"` // 进度配置
	BaseProperties                 // 动态属性
}

// 任务配置数据管理
type QuestCfgMgr struct {
	*DataMap[*QuestCfg] `cfg:"questcfg.csv"`
	progressMgr         *ProgressMgr
	conditionMgr        *ConditionMgr
}

// singleton
func GetQuestCfgMgr() *QuestCfgMgr {
	return _questCfgLoader.Load().(*QuestCfgMgr)
}

func (m *QuestCfgMgr) GetQuestCfg(cfgId int32) *QuestCfg {
	return m.cfgs[cfgId]
}

func (m *QuestCfgMgr) GetProgressMgr() *ProgressMgr {
	return m.progressMgr
}

func (m *QuestCfgMgr) SetProgressMgr(progressMgr *ProgressMgr) {
	m.progressMgr = progressMgr
}

func (m *QuestCfgMgr) GetConditionMgr() *ConditionMgr {
	return m.conditionMgr
}

func (m *QuestCfgMgr) SetConditionMgr(conditionMgr *ConditionMgr) {
	m.conditionMgr = conditionMgr
}
