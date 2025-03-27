package game

import (
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gserver/pb"
)

// 不可叠加的普通物品背包(如限时道具)
type UniqueItemBag struct {
	*UniqueContainer[*pb.UniqueCountItem] `db:""`
}

func NewUniqueItemBag() *UniqueItemBag {
	bag := &UniqueItemBag{
		UniqueContainer: NewBagUnique[*pb.UniqueCountItem](pb.ContainerType_ContainerType_UniqueItem, func(arg *pb.AddElemArg) *pb.UniqueCountItem {
			return &pb.UniqueCountItem{
				CfgId:    arg.GetCfgId(),
				UniqueId: util.GenUniqueId(),
				Timeout:  arg.GetTimeout(),
			}
		}),
	}
	return bag
}
