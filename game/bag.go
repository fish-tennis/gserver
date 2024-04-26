package game

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"math"
)

const (
	// 组件名
	ComponentNameBag = "Bag"
)

// 利用go的init进行组件的自动注册
func init() {
	RegisterPlayerComponentCtor(ComponentNameBag, 100, func(player *Player, playerData *pb.PlayerData) gentity.Component {
		component := &Bag{
			BasePlayerComponent: BasePlayerComponent{
				player: player,
				name:   ComponentNameBag,
			},
			BagCountItem:  NewBagCountItem(),
			BagUniqueItem: NewBagUniqueItem(),
		}
		gentity.LoadData(component, playerData.GetBag())
		return component
	})
}

// 背包模块
// 演示通过组合模式,整合多个不同的子背包模块,提供更高一级的背包接口
type Bag struct {
	BasePlayerComponent
	BagCountItem  *BagCountItem  `child:"CountItem"`
	BagUniqueItem *BagUniqueItem `child:"UniqueItem"`
}

func (this *Player) GetBag() *Bag {
	return this.GetComponentByName(ComponentNameBag).(*Bag)
}

func (this *Bag) AddItem(cfgId int32, num int32) bool {
	itemCfg := cfg.GetItemCfgMgr().GetItemCfg(cfgId)
	if itemCfg == nil {
		return false
	}
	if itemCfg.GetUnique() {
		for i := 0; i < int(num); i++ {
			this.BagUniqueItem.AddUniqueItem(&pb.UniqueItem{
				CfgId:    cfgId,
				UniqueId: util.GenUniqueId(),
			})
		}
	} else {
		this.BagCountItem.AddItem(cfgId, num)
	}
	return true
}

func (this *Bag) AddItems(items []*pb.ItemNum) {
	for _, item := range items {
		this.AddItem(item.CfgId, item.Num)
	}
}

func (this *Bag) DelItems(items []*pb.ItemNum) {
	for _, item := range items {
		itemCfg := cfg.GetItemCfgMgr().GetItemCfg(item.CfgId)
		if itemCfg == nil {
			logger.Debug("itemCfg nil %v", item.CfgId)
			continue
		}
		if itemCfg.GetUnique() {
			this.BagUniqueItem.DelItem(item.CfgId, item.Num)
		} else {
			this.BagCountItem.DelItem(item.CfgId, item.Num)
		}
	}
}

func (this *Bag) IsEnough(items []*pb.ItemNum) bool {
	// items可能有重复的物品,所以转换成map来统计总数量
	itemNumMap := make(map[int32]int64)
	for _, itemNum := range items {
		if itemNum.Num <= 0 {
			logger.Debug("wrong num %v", itemNum)
			return false
		}
		itemNumMap[itemNum.CfgId] += int64(itemNum.Num)
		// 检查int32数值溢出
		if itemNumMap[itemNum.CfgId] > math.MaxInt32 {
			logger.Debug("overflow num %v %v", itemNum, itemNumMap[itemNum.CfgId])
			return false
		}
	}
	for cfgId, num := range itemNumMap {
		itemCfg := cfg.GetItemCfgMgr().GetItemCfg(cfgId)
		if itemCfg == nil {
			logger.Debug("itemCfg nil %v", cfgId)
			return false
		}
		if itemCfg.GetUnique() {
			if this.BagUniqueItem.GetItemCount(cfgId) < int32(num) {
				return false
			}
		} else {
			if this.BagCountItem.GetItemCount(cfgId) < int32(num) {
				return false
			}
		}
	}
	return true
}
