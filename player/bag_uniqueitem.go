package player

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"math/rand"
	"strconv"
)

var _ internal.Saveable = (*BagUniqueItem)(nil)
var _ internal.MapDirtyMark = (*BagUniqueItem)(nil)

// 不可叠加的物品背包
type BagUniqueItem struct {
	MapDataComponent
	items map[int64]*pb.UniqueItem
}

func NewBagUniqueItem(player *Player, data map[int64]*pb.UniqueItem) *BagUniqueItem {
	component := &BagUniqueItem{
		MapDataComponent: *NewMapDataComponent(player, "BagUniqueItem"),
		items: data,
	}
	component.checkData()
	return component
}

func (this *BagUniqueItem) DbData() (dbData interface{}, protoMarshal bool) {
	return this.items,true
}

func (this *BagUniqueItem) CacheData() interface{} {
	return this.items
}

func (this *BagUniqueItem) GetMapValue(key string) (value interface{}, exists bool) {
	value,exists = this.items[util.Atoi64(key)]
	return value,exists
}

func (this *BagUniqueItem) checkData() {
	if this.items == nil {
		this.items = make(map[int64]*pb.UniqueItem)
	}
}

// 事件接口
func (this *BagUniqueItem) OnEvent(event interface{}) {
	switch event.(type) {
	case *internal.EventPlayerEntryGame:
		//// 玩家登录游戏时,先把当前组件数据保存到缓存
		//internal.SaveCache(this, this.GetCacheKey())
		// 测试代码
		uniqueItem := &pb.UniqueItem{UniqueId: util.GenUniqueId(), CfgId: int32(rand.Intn(1000))}
		this.AddUniqueItem(uniqueItem)
	}
}

func (this *BagUniqueItem) AddUniqueItem(uniqueItem *pb.UniqueItem) {
	if _,ok := this.items[uniqueItem.UniqueId]; !ok {
		this.items[uniqueItem.UniqueId] = uniqueItem
		this.SetDirty(strconv.FormatInt(uniqueItem.UniqueId,10), true)
	}
}