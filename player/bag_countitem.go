package player

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/util"
	"math"
	"math/rand"
	"strconv"
)

var _ internal.Saveable = (*BagCountItem)(nil)
var _ internal.MapDirtyMark = (*BagCountItem)(nil)

// 有数量的物品背包
type BagCountItem struct {
	MapDataComponent
	items map[int32]int32
}

func NewBagCountItem(player *Player, data map[int32]int32) *BagCountItem {
	component := &BagCountItem{
		MapDataComponent: *NewMapDataComponent(player, "BagCountItem"),
		items: data,
	}
	component.checkData()
	return component
}

func (this *BagCountItem) DbData() (dbData interface{}, protoMarshal bool) {
	// 演示明文保存数据库
	// 优点:便于查看,数据库语言可直接操作字段
	// 缺点:字段名也会保存到数据库,占用空间多
	return this.items,false
}

func (this *BagCountItem) CacheData() interface{} {
	return this.items
}

func (this *BagCountItem) GetMapValue(key string) (value interface{}, exists bool) {
	value,exists = this.items[int32(util.Atoi(key))]
	return value,exists
}

func (this *BagCountItem) checkData() {
	if this.items == nil {
		this.items = make(map[int32]int32)
	}
}

// 事件接口
func (this *BagCountItem) OnEvent(event interface{}) {
	switch event.(type) {
	case *internal.EventPlayerEntryGame:
		// 测试代码
		this.AddItem(rand.Int31n(100),rand.Int31n(100))
	}
}

func (this *BagCountItem) AddItem(itemCfgId,addCount int32) {
	if addCount <= 0 {
		return
	}
	curCount,ok := this.items[itemCfgId]
	if ok {
		// 检查数值溢出
		if int64(curCount) + int64(addCount) > math.MaxInt32 {
			curCount = math.MaxInt32
		}
	} else {
		curCount = addCount
	}
	this.items[itemCfgId] = curCount
	this.SetDirty(strconv.Itoa(int(itemCfgId)), true)
	logger.Debug("AddItem cfgId:%v curCount:%v", itemCfgId, curCount)
}

func (this *BagCountItem) DelItem(itemCfgId,delCount int32) {
	if delCount <= 0 {
		return
	}
	curCount,ok := this.items[itemCfgId]
	if !ok {
		return
	}
	if delCount >= curCount {
		delete(this.items, itemCfgId)
		this.SetDirty(strconv.Itoa(int(itemCfgId)), false)
	} else {
		this.items[itemCfgId] = curCount - delCount
		this.SetDirty(strconv.Itoa(int(itemCfgId)), true)
	}
	logger.Debug("DelItem cfgId:%v curCount:%v", itemCfgId, curCount)
}