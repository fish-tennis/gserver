package game

import (
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gserver/pb"
)

// 不可叠加的普通物品背包
type BagUniqueItem struct {
	*BagUnique[*pb.UniqueCountItem] `db:""`
}

func NewBagUniqueItem() *BagUniqueItem {
	bag := &BagUniqueItem{
		BagUnique: NewBagUnique[*pb.UniqueCountItem](pb.BagType_BagType_UniqueItem, func(arg *pb.AddItemArg) *pb.UniqueCountItem {
			return &pb.UniqueCountItem{
				CfgId:    arg.GetCfgId(),
				UniqueId: util.GenUniqueId(),
			}
		}),
	}
	return bag
}

func (b *BagUniqueItem) GetCapacity() int32 {
	return 100
}
