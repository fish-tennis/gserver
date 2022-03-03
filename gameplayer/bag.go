package gameplayer

import (
	"github.com/fish-tennis/gserver/internal"
)

// 背包模块
// 演示通过组合模式,整合多个不同的子背包模块,提供更高一级的背包接口
type Bag struct {
	BaseComponent
	bagCountItem *BagCountItem
	bagUniqueItem *BagUniqueItem
}

func NewBag(player *Player, bagCountItem *BagCountItem, bagUniqueItem *BagUniqueItem) *Bag {
	component := &Bag{
		BaseComponent: BaseComponent{
			Player: player,
			Name: "Bag",
		},
		bagCountItem: bagCountItem,
		bagUniqueItem: bagUniqueItem,
	}
	return component
}

// 事件接口
func (this *Bag) OnEvent(event interface{}) {
	switch event.(type) {
	case *internal.EventPlayerEntryGame:
		// 测试代码
	}
}