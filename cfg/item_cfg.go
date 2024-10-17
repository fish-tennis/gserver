package cfg

import (
	"github.com/fish-tennis/gserver/gen"
	"github.com/fish-tennis/gserver/pb"
)

var (
	_itemCfgLoader = Register(func() any {
		return new(ItemCfgMgr)
	}, First)
)

// 任务配置数据管理
type ItemCfgMgr struct {
	*DataMap[*pb.ItemCfg] `cfg:"itemcfg.csv"`
}

// singleton
func GetItemCfgMgr() *ItemCfgMgr {
	return _itemCfgLoader.Load().(*ItemCfgMgr)
}

// 提供一个只读接口
func (m *ItemCfgMgr) GetItemCfg(cfgId int32) *gen.ItemCfgR {
	return gen.NewItemCfgR(m.GetCfg(cfgId))
}
