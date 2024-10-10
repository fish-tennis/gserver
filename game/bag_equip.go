package game

import (
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gserver/pb"
)

// 装备背包
type BagEquip struct {
	*BagUnique[*pb.Equip] `db:""`
}

func NewBagEquip() *BagEquip {
	bag := &BagEquip{
		BagUnique: NewBagUnique[*pb.Equip](func(arg *pb.AddItemArg) *pb.Equip {
			return &pb.Equip{
				CfgId:    arg.GetCfgId(),
				UniqueId: util.GenUniqueId(),
			}
		}),
	}
	return bag
}
