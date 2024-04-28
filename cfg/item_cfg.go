package cfg

import (
	"encoding/json"
	"github.com/fish-tennis/gserver/gen"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"os"
)

var (
	_itemCfgMgr = &ItemCfgMgr{
		cfgs: make(map[int32]*pb.ItemCfg),
	}
)

func init() {
	RegisterCfgLoader(&CfgLoaderOption{
		Loader:   _itemCfgMgr,
		FileName: "itemcfg.json",
	})
}

// 任务配置数据管理
type ItemCfgMgr struct {
	cfgs map[int32]*pb.ItemCfg
}

func GetItemCfgMgr() *ItemCfgMgr {
	return _itemCfgMgr
}

func (this *ItemCfgMgr) GetItemCfg(cfgId int32) *gen.ItemCfgReader {
	return gen.NewItemCfgReader(this.cfgs[cfgId])
}

func (this *ItemCfgMgr) Load(fileName string) bool {
	fileData, err := os.ReadFile(fileName)
	if err != nil {
		logger.Error("%v", err)
		return false
	}
	var cfgList []*pb.ItemCfg
	err = json.Unmarshal(fileData, &cfgList)
	if err != nil {
		logger.Error("%v", err)
		return false
	}
	cfgMap := make(map[int32]*pb.ItemCfg, len(cfgList))
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
