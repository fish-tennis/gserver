package game

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/logger"
	"math"
)

// 有数量的物品背包
type BagCountItem struct {
	gentity.BaseMapDirtyMark
	Items map[int32]int32 `db:"CountItem;plain"`
}

func NewBagCountItem() *BagCountItem {
	bag := &BagCountItem{
		Items: make(map[int32]int32),
	}
	return bag
}

func (this *BagCountItem) AddItem(itemCfgId, addCount int32) {
	if addCount <= 0 {
		return
	}
	curCount, ok := this.Items[itemCfgId]
	if ok {
		// 检查数值溢出
		if int64(curCount)+int64(addCount) > math.MaxInt32 {
			curCount = math.MaxInt32
		} else {
			curCount += addCount
		}
	} else {
		curCount = addCount
	}
	this.Items[itemCfgId] = curCount
	this.SetDirty(itemCfgId, true)
	logger.Debug("AddItem cfgId:%v curCount:%v", itemCfgId, curCount)
}

func (this *BagCountItem) DelItem(itemCfgId, delCount int32) {
	if delCount <= 0 {
		return
	}
	curCount, ok := this.Items[itemCfgId]
	if !ok {
		return
	}
	if delCount >= curCount {
		delete(this.Items, itemCfgId)
		this.SetDirty(itemCfgId, false)
	} else {
		this.Items[itemCfgId] = curCount - delCount
		this.SetDirty(itemCfgId, true)
	}
	logger.Debug("DelItem cfgId:%v curCount:%v", itemCfgId, curCount)
}
