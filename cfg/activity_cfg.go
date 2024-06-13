package cfg

import (
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
)

var (
	_activityCfgMgr = &ActivityCfgMgr{}
)

func init() {
	RegisterCfgLoader(&CfgLoaderOption{
		Loader:   _activityCfgMgr,
		FileName: "activitycfg.json",
	})
}

// 活动配置数据
type ActivityCfg struct {
	pb.BaseActivityCfg                        // 活动基础数据
	Quests             []*QuestCfg            // 一个活动可以包含N个子任务
	Properties         map[string]interface{} `json:"Properties"` // 动态属性
}

func (this *ActivityCfg) GetQuestCfg(cfgId int32) *QuestCfg {
	for _, questCfg := range this.Quests {
		if questCfg.GetCfgId() == cfgId {
			return questCfg
		}
	}
	return nil
}

// 活动配置数据管理
type ActivityCfgMgr struct {
	DataMap[*ActivityCfg]
	progressMgr  *ProgressMgr
	conditionMgr *ConditionMgr
}

// singleton
func GetActivityCfgMgr() *ActivityCfgMgr {
	return _activityCfgMgr
}

func (this *ActivityCfgMgr) GetActivityCfg(cfgId int32) *ActivityCfg {
	return this.cfgs[cfgId]
}

func (this *ActivityCfgMgr) GetProgressMgr() *ProgressMgr {
	return this.progressMgr
}

func (this *ActivityCfgMgr) SetProgressMgr(progressMgr *ProgressMgr) {
	this.progressMgr = progressMgr
}

func (this *ActivityCfgMgr) GetConditionMgr() *ConditionMgr {
	return this.conditionMgr
}

func (this *ActivityCfgMgr) SetConditionMgr(conditionMgr *ConditionMgr) {
	this.conditionMgr = conditionMgr
}
