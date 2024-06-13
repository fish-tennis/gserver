package cfg

import (
	"github.com/fish-tennis/gserver/gen"
	"github.com/fish-tennis/gserver/pb"
)

var (
	_itemCfgMgr = &ItemCfgMgr{}
)

func init() {
	RegisterCfgLoader(&CfgLoaderOption{
		Loader:   _itemCfgMgr,
		FileName: "itemcfg.json",
	})
}

// 任务配置数据管理
type ItemCfgMgr struct {
	DataMap[*pb.ItemCfg]
}

// singleton
func GetItemCfgMgr() *ItemCfgMgr {
	return _itemCfgMgr
}

// 提供一个只读接口
func (this *ItemCfgMgr) GetItemCfg(cfgId int32) *gen.ItemCfgReader {
	return gen.NewItemCfgReader(this.GetCfg(cfgId))
}
