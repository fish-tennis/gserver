package cfg

import (
	"encoding/json"
	"github.com/fish-tennis/gserver/logger"
	"os"
)

var (
	_itemCfgMgr *ItemCfgMgr
)

// 物品配置数据
type ItemCfg struct {
	CfgId  int32  `json:"cfgId"` // 配置id
	Name   string `json:"name"`
	Detail string `json:"detail"` // 详情
	Unique bool   `json:"unique"` // 是否不可叠加
}

// 任务配置数据管理
type ItemCfgMgr struct {
	cfgs map[int32]*ItemCfg
}

func GetItemCfgMgr() *ItemCfgMgr {
	if _itemCfgMgr == nil {
		_itemCfgMgr = &ItemCfgMgr{
			cfgs: make(map[int32]*ItemCfg),
		}
	}
	return _itemCfgMgr
}

func (this *ItemCfgMgr) GetItemCfg(cfgId int32) *ItemCfg {
	return this.cfgs[cfgId]
}

func (this *ItemCfgMgr) Load(fileName string) bool {
	fileData, err := os.ReadFile(fileName)
	if err != nil {
		logger.Error("%v", err)
		return false
	}
	var cfgList []*ItemCfg
	err = json.Unmarshal(fileData, &cfgList)
	if err != nil {
		logger.Error("%v", err)
		return false
	}
	cfgMap := make(map[int32]*ItemCfg, len(cfgList))
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
