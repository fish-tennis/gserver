package game

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
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

func (b *BagUniqueItem) GetItemCount(itemCfgId int32) int32 {
	itemCount := int32(0)
	for _, item := range b.Data {
		if item.CfgId == itemCfgId {
			itemCount++
		}
	}
	return itemCount
}

func (b *BagUniqueItem) AddUniqueItem(uniqueItem *pb.UniqueItem) int32 {
	if len(b.Data) >= int(b.GetCapacity()) {
		logger.Debug("BagFull cfgId:%v uniqueId:%v", uniqueItem.GetCfgId(), uniqueItem.GetUniqueId())
		return 0
	}
	if _, ok := b.Data[uniqueItem.UniqueId]; !ok {
		b.Set(uniqueItem.UniqueId, uniqueItem)
		logger.Debug("AddUniqueItem CfgId:%v UniqueId:%v", uniqueItem.CfgId, uniqueItem.UniqueId)
		return 1
	}
	return 0
}

func (b *BagUniqueItem) DelUniqueItem(uniqueId int64) int32 {
	if _, ok := b.Data[uniqueId]; ok {
		b.Delete(uniqueId)
		logger.Debug("DelUniqueItem UniqueId:%v", uniqueId)
		return 1
	}
	return 0
}

func (b *BagUniqueItem) GetCapacity() int32 {
	return 100
}

func (b *BagUniqueItem) AddItem(itemCfgId, addCount int32) int32 {
	if addCount <= 0 {
		return 0
	}
	for i := 0; i < int(addCount); i++ {
		if len(b.Data) >= int(b.GetCapacity()) {
			logger.Debug("BagFull cfgId:%v addCount:%v realAddCount", itemCfgId, addCount, i)
			return int32(i)
		}
		b.AddUniqueItem(&pb.UniqueItem{
			CfgId:    itemCfgId,
			UniqueId: util.GenUniqueId(),
		})
	}
	return addCount
}

func (b *BagUniqueItem) DelItem(itemCfgId, delCount int32) int32 {
	realDelCount := int32(0)
	for _, item := range b.Data {
		if item.CfgId == itemCfgId {
			b.Delete(item.UniqueId)
			realDelCount++
			logger.Debug("DelItem cfgId:%v UniqueId:%v", itemCfgId, item.UniqueId)
			if realDelCount >= delCount {
				break
			}
		}
	}
	return realDelCount
}
