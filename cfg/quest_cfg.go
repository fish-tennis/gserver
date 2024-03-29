package cfg

import (
	"encoding/json"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"os"
)

var (
	_questCfgMgr *QuestCfgMgr
)

// 任务配置数据
type QuestCfg struct {
	pb.BaseQuestCfg
	Conditions     []*ConditionCfg `json:"Conditions"`  // 条件配置
	ProgressCfg    *ProgressCfg    `json:"ProgressCfg"` // 进度配置
	BaseProperties                                      // 动态属性
}

// 任务配置数据管理
type QuestCfgMgr struct {
	cfgs         map[int32]*QuestCfg
	progressMgr  *ProgressMgr
	conditionMgr *ConditionMgr
}

func GetQuestCfgMgr() *QuestCfgMgr {
	if _questCfgMgr == nil {
		_questCfgMgr = &QuestCfgMgr{
			cfgs: make(map[int32]*QuestCfg),
		}
	}
	return _questCfgMgr
}

func (this *QuestCfgMgr) GetQuestCfg(cfgId int32) *QuestCfg {
	return this.cfgs[cfgId]
}

func (this *QuestCfgMgr) Range(f func(questCfg *QuestCfg) bool) {
	for _, cfg := range this.cfgs {
		if !f(cfg) {
			return
		}
	}
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

// 加载任务配置数据
func (this *QuestCfgMgr) Load(fileName string) bool {
	fileData, err := os.ReadFile(fileName)
	if err != nil {
		logger.Error("%v", err)
		return false
	}
	var cfgList []*QuestCfg
	err = json.Unmarshal(fileData, &cfgList)
	if err != nil {
		logger.Error("%v", err)
		return false
	}
	cfgMap := make(map[int32]*QuestCfg, len(cfgList))
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
