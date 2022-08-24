package gameplayer

import (
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/pb"
)

// 背包模块
// 演示通过组合模式,整合多个不同的子背包模块,提供更高一级的背包接口
type Bag struct {
	BasePlayerComponent
	BagCountItem  *BagCountItem  `child:"CountItem"`
	BagUniqueItem *BagUniqueItem `child:"UniqueItem"`
}

func NewBag(player *Player) *Bag {
	component := &Bag{
		BasePlayerComponent: BasePlayerComponent{
			player: player,
			name:   "Bag",
		},
		BagCountItem:  NewBagCountItem(),
		BagUniqueItem: NewBagUniqueItem(),
	}
	return component
}

func (this *Bag) AddItem(cfgId int32, num int32) bool {
	itemCfg := cfg.GetItemCfgMgr().GetItemCfg(cfgId)
	if itemCfg == nil {
		return false
	}
	if itemCfg.Unique {
		for i := 0; i < int(num); i++ {
			this.BagUniqueItem.AddUniqueItem(&pb.UniqueItem{
				CfgId: cfgId,
				UniqueId: util.GenUniqueId(),
			})
		}
	} else {
		this.BagCountItem.AddItem(cfgId, num)
	}
	return true
}