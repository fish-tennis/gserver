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
	// 如果有多条件的任务需求,可以改成[]*ConditionCfg
	ConditionCfg *ConditionCfg `json:"ConditionCfg"` // 条件配置
}

// 任务配置数据管理
type QuestCfgMgr struct {
	cfgs         map[int32]*QuestCfg
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

func (this *QuestCfgMgr) GetQuestCfgs() map[int32]*QuestCfg {
	return this.cfgs
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
