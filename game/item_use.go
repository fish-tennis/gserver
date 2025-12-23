package game

import (
	"errors"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
)

type ItemUseArgs struct {
	CfgId int32             // 物品配置id
	Item  internal.Uniquely // 物品对象(唯一物品才有)
}

// 物品使用接口
type ItemUseFunc func(player *Player, itemCfg *pb.ItemCfg, useArgs *ItemUseArgs) error

var (
	// 根据物品Id注册的物品使用接口(优先查找该表)
	_itemUseRegisterByItemId = map[int32]ItemUseFunc{}
	// 根据ItemSubType注册的物品使用接口
	_itemUseRegisterByItemSubType = map[int32]ItemUseFunc{}
)

// 注册物品使用接口
func init() {
	_itemUseRegisterByItemSubType[int32(pb.ItemSubType_ItemSubType_Exp)] = UseItem_Exp
}

// 加经验的道具
func UseItem_Exp(player *Player, itemCfg *pb.ItemCfg, useArgs *ItemUseArgs) error {
	if len(itemCfg.GetArgs()) == 0 {
		return errors.New("ArgsError")
	}
	addExp := itemCfg.GetArgs()[0]
	if addExp <= 0 {
		return errors.New("ArgError")
	}
	player.GetBaseInfo().IncExp(addExp)
	return nil
}
