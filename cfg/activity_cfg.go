package cfg

import (
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
)

var (
	_activityCfgLoader = Register(func() any {
		return new(ActivityCfgMgr)
	}, Mid+1) // 任务配置加载完,再加载活动配置
)

//// 活动配置数据
//type ActivityCfg struct {
//	pb.BaseActivityCfg                // 活动基础数据
//	Properties         map[string]any `json:"Properties"` // 动态属性
//}

//func (c *ActivityCfg) GetQuestCfg(cfgId int32) *QuestCfg {
//	if !slices.Contains(c.Quests, cfgId) {
//		return nil
//	}
//	return GetQuestCfgMgr().GetQuestCfg(cfgId)
//}

// 活动配置数据管理
type ActivityCfgMgr struct {
	*DataMap[*pb.ActivityCfg] `cfg:"activitycfg.csv"`
	progressMgr               *ProgressMgr
	conditionMgr              *ConditionMgr
}

// singleton
func GetActivityCfgMgr() *ActivityCfgMgr {
	return _activityCfgLoader.Load().(*ActivityCfgMgr)
}

func (m *ActivityCfgMgr) GetActivityCfg(cfgId int32) *pb.ActivityCfg {
	return m.cfgs[cfgId]
}

func (m *ActivityCfgMgr) GetProgressMgr() *ProgressMgr {
	return m.progressMgr
}

func (m *ActivityCfgMgr) SetProgressMgr(progressMgr *ProgressMgr) {
	m.progressMgr = progressMgr
}

func (m *ActivityCfgMgr) GetConditionMgr() *ConditionMgr {
	return m.conditionMgr
}

func (m *ActivityCfgMgr) SetConditionMgr(conditionMgr *ConditionMgr) {
	m.conditionMgr = conditionMgr
}
