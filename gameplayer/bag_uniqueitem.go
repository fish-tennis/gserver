package gameplayer

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

var _ internal.Saveable = (*BagUniqueItem)(nil)
var _ internal.MapDirtyMark = (*BagUniqueItem)(nil)

// 不可叠加的物品背包
type BagUniqueItem struct {
	internal.BaseMapDirtyMark
	Items map[int64]*pb.UniqueItem `db:"uniqueitems"`
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
