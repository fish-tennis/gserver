package game

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
	"math"
	"time"
)

const (
	// 组件名
	ComponentNameBag = "Bags"
)

// 利用go的init进行组件的自动注册
func init() {
	_playerComponentRegister.Register(ComponentNameBag, 100, func(player *Player, _ any) gentity.Component {
		return &Bags{
			BasePlayerComponent: BasePlayerComponent{
				player: player,
				name:   ComponentNameBag,
			},
			BagCountItem:  NewBagCountItem(),
			BagUniqueItem: NewBagUniqueItem(),
			BagEquip:      NewBagEquip(),
		}
	})
}

// 背包模块
// 演示通过组合模式,整合多个不同的子背包模块,提供更高一级的背包接口
type Bags struct {
	BasePlayerComponent
	BagCountItem  *BagCountItem  `child:"CountItem"`  // 普通物品
	BagUniqueItem *BagUniqueItem `child:"UniqueItem"` // 不可叠加的普通物品(如限时类的普通物品)
	BagEquip      *BagEquip      `child:"Equip"`      // 装备
}

func (p *Player) GetBags() *Bags {
	return p.GetComponentByName(ComponentNameBag).(*Bags)
}

func (b *Bags) SyncDataToClient() {
	b.GetPlayer().Send(&pb.BagsSync{
		CountItem:  b.BagCountItem.Data,
		UniqueItem: b.BagUniqueItem.Data,
		Equip:      b.BagEquip.Data,
	})
}

// 根据物品配置获取对应的子背包
func (b *Bags) GetBag(itemCfgId int32) internal.Bag {
	return b.GetBagByArg(&pb.AddItemArg{
		CfgId: itemCfgId,
	})
}

func (b *Bags) GetBagByArg(arg *pb.AddItemArg) internal.Bag {
	itemCfg := cfg.GetItemCfgMgr().GetItemCfg(arg.GetCfgId())
	if itemCfg == nil {
		slog.Error("ErrItemCfgId", "itemCfgId", arg.GetCfgId())
		return nil
	}
	switch itemCfg.GetItemType() {
	case int32(pb.ItemType_ItemType_None):
		// 限时道具
		if itemCfg.GetTimeType() > 0 || arg.GetTimeType() > 0 {
			// 有限时属性的普通物品,变成不可叠加的
			return b.BagUniqueItem
		}
		return b.BagCountItem
	case int32(pb.ItemType_ItemType_Equip):
		return b.BagEquip
	}
	slog.Error("ErrItemType", "itemCfgId", itemCfg.GetCfgId(), "itemType", itemCfg.GetItemType())
	return nil
}

func (b *Bags) GetItemCount(itemCfgId int32) int32 {
	bag := b.GetBag(itemCfgId)
	if bag == nil {
		return 0
	}
	return bag.GetItemCount(itemCfgId)
}

// 背包模块提供对外的添加物品接口
// 业务层应该尽量使用该接口
func (b *Bags) AddItem(arg *pb.AddItemArg, bagUpdate *pb.BagUpdate) int32 {
	bag := b.GetBagByArg(arg)
	if bag == nil {
		return 0
	}
	return bag.AddItem(arg, bagUpdate)
}

func (b *Bags) AddItems(addItemArgs []*pb.AddItemArg) int32 {
	bagUpdate := &pb.BagUpdate{}
	total := int32(0)
	for _, addItemArg := range addItemArgs {
		total += b.AddItem(addItemArg, bagUpdate)
	}
	if len(bagUpdate.ItemOps) > 0 {
		b.GetPlayer().Send(bagUpdate) // 同步背包变化给客户端
		//slog.Info("AddItems", "bagUpdate", bagUpdate)
	}
	return total
}

func (b *Bags) AddItemById(cfgId, num int32) int32 {
	bagUpdate := &pb.BagUpdate{}
	return b.AddItem(&pb.AddItemArg{
		CfgId: cfgId,
		Num:   num,
	}, bagUpdate)
}

func (b *Bags) DelItems(delItems []*pb.DelItemArg) int32 {
	bagUpdate := &pb.BagUpdate{}
	total := int32(0)
	for _, delItem := range delItems {
		bag := b.GetBag(delItem.CfgId)
		if bag == nil {
			slog.Debug("bag is nil", "cfgId", delItem.CfgId)
			continue
		}
		total += bag.DelItem(delItem, bagUpdate)
	}
	if len(bagUpdate.ItemOps) > 0 {
		b.GetPlayer().Send(bagUpdate) // 同步背包变化给客户端
		//slog.Info("DelItems", "bagUpdate", bagUpdate)
	}
	return total
}

func (b *Bags) IsEnough(items []*pb.DelItemArg) bool {
	// items可能有重复的物品,所以转换成map来统计总数量
	itemNumMap := make(map[int32]int64)
	for _, itemNum := range items {
		if itemNum.Num <= 0 {
			slog.Debug("wrong num", "itemNum", itemNum)
			return false
		}
		itemNumMap[itemNum.CfgId] += int64(itemNum.Num)
		// 检查int32数值溢出
		if itemNumMap[itemNum.CfgId] > math.MaxInt32 {
			slog.Debug("overflow num", "cfgId", itemNum.CfgId, "itemNum", itemNum, "total", itemNumMap[itemNum.CfgId])
			return false
		}
	}
	for cfgId, num := range itemNumMap {
		bag := b.GetBag(cfgId)
		if bag == nil {
			slog.Debug("bag is nil", "cfgId", cfgId)
			return false
		}
		if bag.GetItemCount(cfgId) < int32(num) {
			return false
		}
	}
	return true
}

// 响应事件:玩家进入游戏
func (b *Bags) TriggerPlayerEntryGame(event *internal.EventPlayerEntryGame) {
	b.BagUniqueItem.initTimeoutList()
	b.BagEquip.initTimeoutList()
	// 超时检查回调
	b.GetPlayer().GetTimerEntries().After(time.Second, func() time.Duration {
		bagUpdate := &pb.BagUpdate{}
		now := int32(b.GetPlayer().GetTimerEntries().Now().Unix())
		b.BagUniqueItem.checkTimeout(now, bagUpdate)
		b.BagEquip.checkTimeout(now, bagUpdate)
		if len(bagUpdate.ItemOps) > 0 {
			b.GetPlayer().Send(bagUpdate) // 同步背包变化给客户端
			//slog.Info("checkTimeout", "bagUpdate", bagUpdate)
		}
		return time.Second
	})
}
