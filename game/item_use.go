package game

import (
	"errors"
	"math"

	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
)

const (
	ErrItemArgsError = "ItemArgsError"
)

type ItemUseArgs struct {
	CfgId int32             // 物品配置id
	Item  internal.Uniquely // 物品对象(唯一物品才有)
	Num   int32             // 使用数量
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
		return errors.New(ErrItemArgsError)
	}
	addExp := itemCfg.GetArgs()[0]
	if addExp <= 0 {
		return errors.New(ErrItemArgsError)
	}
	// 按使用数量成倍加经验,使用 int64 乘法防溢出
	totalExp := int64(addExp) * int64(useArgs.Num)
	if totalExp > math.MaxInt32 {
		totalExp = math.MaxInt32
	}
	player.GetBaseInfo().IncExp(int32(totalExp))
	player.Log.Debug("UseItem_Exp", "addExp", totalExp)
	return nil
}
