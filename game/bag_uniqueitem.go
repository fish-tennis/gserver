package game

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

// 不可叠加的物品背包
type BagUniqueItem struct {
	*gentity.MapData[int64, *pb.UniqueItem] `db:""`
}

func NewBagUniqueItem() *BagUniqueItem {
	bag := &BagUniqueItem{
		MapData: gentity.NewMapData[int64, *pb.UniqueItem](),
	}
	return bag
}

func (this *BagUniqueItem) GetItemCount(itemCfgId int32) int32 {
	itemCount := int32(0)
	for _, item := range this.Data {
		if item.CfgId == itemCfgId {
			itemCount++
		}
	}
	return itemCount
}

func (this *BagUniqueItem) AddUniqueItem(uniqueItem *pb.UniqueItem) int32 {
	if _, ok := this.Data[uniqueItem.UniqueId]; !ok {
		this.Set(uniqueItem.UniqueId, uniqueItem)
		logger.Debug("AddUniqueItem CfgId:%v UniqueId:%v", uniqueItem.CfgId, uniqueItem.UniqueId)
		return 1
	}
	return 0
}

func (this *BagUniqueItem) DelUniqueItem(uniqueId int64) int32 {
	if _, ok := this.Data[uniqueId]; ok {
		this.Delete(uniqueId)
		logger.Debug("DelUniqueItem UniqueId:%v", uniqueId)
		return 1
	}
	return 0
}

func (this *BagUniqueItem) DelItem(itemCfgId, delCount int32) int32 {
	realDelCount := int32(0)
	for _, item := range this.Data {
		if item.CfgId == itemCfgId {
			this.Delete(item.UniqueId)
			realDelCount++
			logger.Debug("DelItem cfgId:%v UniqueId:%v", itemCfgId, item.UniqueId)
			if realDelCount >= delCount {
				break
			}
		}
	}
	return realDelCount
}
