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

func (this *BagUniqueItem) GetItemCount(itemCfgId int32) int32 {
	itemCount := int32(0)
	for _,item := range this.Items {
		if item.CfgId == itemCfgId {
			itemCount++
		}
	}
	return itemCount
}

func (this *BagUniqueItem) AddUniqueItem(uniqueItem *pb.UniqueItem) int32 {
	if _, ok := this.Items[uniqueItem.UniqueId]; !ok {
		this.Items[uniqueItem.UniqueId] = uniqueItem
		this.SetDirty(uniqueItem.UniqueId, true)
		logger.Debug("AddUniqueItem CfgId:%v UniqueId:%v", uniqueItem.CfgId, uniqueItem.UniqueId)
		return 1
	}
	return 0
}

func (this *BagUniqueItem) DelUniqueItem(uniqueId int64) int32 {
	if _, ok := this.Items[uniqueId]; ok {
		delete(this.Items, uniqueId)
		this.SetDirty(uniqueId, true)
		logger.Debug("DelUniqueItem UniqueId:%v", uniqueId)
		return 1
	}
	return 0
}

func (this *BagUniqueItem) DelItem(itemCfgId, delCount int32) int32 {
	realDelCount := int32(0)
	for _,item := range this.Items {
		if item.CfgId == itemCfgId {
			delete(this.Items, item.UniqueId)
			this.SetDirty(item.UniqueId, false)
			realDelCount++
			logger.Debug("DelItem cfgId:%v UniqueId:%v", itemCfgId, item.UniqueId)
		}
	}
	return realDelCount
}
