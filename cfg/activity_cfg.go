package cfg

import (
	"encoding/json"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"os"
)

var (
	_activityCfgMgr = &ActivityCfgMgr{
		cfgs: make(map[int32]*ActivityCfg),
	}
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
	cfgs         map[int32]*ActivityCfg
	progressMgr  *ProgressMgr
	conditionMgr *ConditionMgr
}

func GetActivityCfgMgr() *ActivityCfgMgr {
	return _activityCfgMgr
}

func (this *ActivityCfgMgr) GetActivityCfg(cfgId int32) *ActivityCfg {
	return this.cfgs[cfgId]
}

func (this *ActivityCfgMgr) Range(f func(activityCfg *ActivityCfg) bool) {
	for _, cfg := range this.cfgs {
		if !f(cfg) {
			return
		}
	}
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

// 加载任务配置数据
func (this *ActivityCfgMgr) Load(fileName string) bool {
	fileData, err := os.ReadFile(fileName)
	if err != nil {
		logger.Error("%v", err)
		return false
	}
	var cfgList []*ActivityCfg
	err = json.Unmarshal(fileData, &cfgList)
	if err != nil {
		logger.Error("%v", err)
		return false
	}
	cfgMap := make(map[int32]*ActivityCfg, len(cfgList))
	for _, cfg := range cfgList {
		if _, exists := cfgMap[cfg.CfgId]; exists {
			logger.Error("duplicate id:%v", cfg.CfgId)
		}
		cfgMap[cfg.CfgId] = cfg
	}
	this.cfgs = cfgMap
	logger.Info("count:%v", len(this.cfgs))
	return true
}
