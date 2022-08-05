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
	PlayerMapDataComponent
	Items map[int64]*pb.UniqueItem `db:"baguniqueitem"`
}

func NewBagUniqueItem(player *Player) *BagUniqueItem {
	component := &BagUniqueItem{
		PlayerMapDataComponent: *NewPlayerMapDataComponent(player, "BagUniqueItem"),
		Items:                  make(map[int64]*pb.UniqueItem),
	}
	component.checkData()
	return component
}

func (this *BagUniqueItem) checkData() {
	if this.Items == nil {
		this.Items = make(map[int64]*pb.UniqueItem)
	}
}

// 事件接口
func (this *BagUniqueItem) OnEvent(event interface{}) {
	switch event.(type) {
	case *internal.EventPlayerEntryGame:
		//// 测试代码
		//uniqueItem := &pb.UniqueItem{UniqueId: util.GenUniqueId(), CfgId: int32(rand.Intn(1000))}
		//this.AddUniqueItem(uniqueItem)
	}
}

func (this *BagUniqueItem) AddUniqueItem(uniqueItem *pb.UniqueItem) {
	if _,ok := this.Items[uniqueItem.UniqueId]; !ok {
		this.Items[uniqueItem.UniqueId] = uniqueItem
		this.SetDirty(uniqueItem.UniqueId, true)
		logger.Debug("AddUniqueItem CfgId:%v UniqueId:%v", uniqueItem.CfgId, uniqueItem.UniqueId)
	}
}