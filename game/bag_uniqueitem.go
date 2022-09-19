package game

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

// 不可叠加的物品背包
type BagUniqueItem struct {
	gentity.BaseMapDirtyMark
	Items map[int64]*pb.UniqueItem `db:"UniqueItem"`
}

func NewBagUniqueItem() *BagUniqueItem {
	bag := &BagUniqueItem{
		Items: make(map[int64]*pb.UniqueItem),
	}
	return bag
}

func (this *BagUniqueItem) AddUniqueItem(uniqueItem *pb.UniqueItem) {
	if _, ok := this.Items[uniqueItem.UniqueId]; !ok {
		this.Items[uniqueItem.UniqueId] = uniqueItem
		this.SetDirty(uniqueItem.UniqueId, true)
		logger.Debug("AddUniqueItem CfgId:%v UniqueId:%v", uniqueItem.CfgId, uniqueItem.UniqueId)
	}
}
