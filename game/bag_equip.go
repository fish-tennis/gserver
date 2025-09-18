package game

import (
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gserver/pb"
)

// 装备背包(这里演示的普通的装备背包,像RPG那种能拖动格子的背包需要另行实现)
type EquipBag struct {
	*UniqueContainer[*pb.Equip] `db:""`
}

func NewBagEquip(bags *Bags) *EquipBag {
	bag := &EquipBag{
		UniqueContainer: NewBagUnique[*pb.Equip](bags, pb.ContainerType_ContainerType_Equip, func(arg *pb.AddElemArg) *pb.Equip {
			return &pb.Equip{
				CfgId:    arg.GetCfgId(),
				UniqueId: util.GenUniqueId(),
				Timeout:  arg.GetTimeout(),
			}
		}),
	}
	return bag
}
