package cfg

import (
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
)

var (
	_questCfgLoader *CfgLoaderOption
)

func init() {
	_questCfgLoader = RegisterCfgLoader(new(QuestCfgMgr), &CfgLoaderOption{
		FileName: "questcfg.json",
	})
}

// 任务配置数据
type QuestCfg struct {
	pb.BaseQuestCfg
	Conditions     []*ConditionCfg `json:"Conditions"`  // 条件配置
	ProgressCfg    *ProgressCfg    `json:"ProgressCfg"` // 进度配置
	BaseProperties                 // 动态属性
}

// 任务配置数据管理
type QuestCfgMgr struct {
	DataMap[*QuestCfg]
	progressMgr  *ProgressMgr
	conditionMgr *ConditionMgr
}

// singleton
func GetQuestCfgMgr() *QuestCfgMgr {
	return _questCfgLoader.Value.Load().(*QuestCfgMgr)
}

func (this *QuestCfgMgr) GetQuestCfg(cfgId int32) *QuestCfg {
	return this.cfgs[cfgId]
}

func (this *QuestCfgMgr) GetProgressMgr() *ProgressMgr {
	return this.progressMgr
}

func (this *QuestCfgMgr) SetProgressMgr(progressMgr *ProgressMgr) {
	this.progressMgr = progressMgr
}

func (this *QuestCfgMgr) GetConditionMgr() *ConditionMgr {
	return this.conditionMgr
}

func (this *QuestCfgMgr) SetConditionMgr(conditionMgr *ConditionMgr) {
	this.conditionMgr = conditionMgr
}
