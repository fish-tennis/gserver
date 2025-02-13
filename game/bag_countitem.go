package game

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"math"
)

// 有数量的物品背包
type BagCountItem struct {
	*gentity.MapData[int32, int32] `db:""`
	bagType                        pb.BagType
}

func NewBagCountItem() *BagCountItem {
	bag := &BagCountItem{
		MapData: gentity.NewMapData[int32, int32](),
		bagType: pb.BagType_BagType_CountItem,
	}
	return bag
}

// 背包容量
func (b *BagCountItem) GetCapacity() int32 {
	// 实际项目读取配置或者是个变量,比如可以扩展背包容量
	return 100
}

func (b *BagCountItem) GetItemCount(itemCfgId int32) int32 {
	return b.Data[itemCfgId]
}

func (b *BagCountItem) AddItem(arg *pb.AddItemArg, bagUpdate *pb.BagUpdate) int32 {
	addCount := arg.GetNum()
	if addCount <= 0 {
		return 0
	}
	curCount, ok := b.Data[arg.GetCfgId()]
	if ok {
		// 检查数值溢出
		if int64(curCount)+int64(addCount) > math.MaxInt32 {
			addCount = math.MaxInt32 - curCount
			curCount = math.MaxInt32
		} else {
			curCount += addCount
		}
	} else {
		if len(b.Data) >= int(b.GetCapacity()) {
			logger.Debug("BagFull cfgId:%v addCount:%v", arg.GetCfgId(), addCount)
			return 0
		}
		curCount = addCount
	}
	b.Set(arg.GetCfgId(), curCount)
	logger.Debug("AddItem cfgId:%v curCount:%v addCount:%v", arg.GetCfgId(), curCount, addCount)
	if bagUpdate != nil && addCount > 0 {
		itemOp := &pb.BagItemOp{
			BagType: b.bagType,
			OpType:  pb.BagItemOpType_BagItemOpType_Add,
			BagItem: &pb.BagItemOp_CountItem{
				CountItem: &pb.ItemNum{
					CfgId: arg.GetCfgId(),
					Num:   addCount,
				},
			},
		}
		bagUpdate.ItemOps = append(bagUpdate.ItemOps, itemOp)
	}
	return addCount
}

func (b *BagCountItem) DelItem(arg *pb.DelItemArg, bagUpdate *pb.BagUpdate) int32 {
	delCount := arg.GetNum()
	if delCount <= 0 {
		return 0
	}
	curCount, ok := b.Data[arg.GetCfgId()]
	if !ok {
		return 0
	}
	realDelCount := delCount
	if delCount >= curCount {
		b.Delete(arg.GetCfgId())
		logger.Debug("DelItem cfgId:%v delCount:%v/%v", arg.GetCfgId(), curCount, delCount)
		realDelCount = curCount
	} else {
		b.Set(arg.GetCfgId(), curCount-delCount)
		logger.Debug("DelItem cfgId:%v delCount:%v", arg.GetCfgId(), delCount)
	}
	if bagUpdate != nil {
		itemOp := &pb.BagItemOp{
			BagType: b.bagType,
			OpType:  pb.BagItemOpType_BagItemOpType_Delete,
			BagItem: &pb.BagItemOp_CountItem{
				CountItem: &pb.ItemNum{
					CfgId: arg.GetCfgId(),
					Num:   realDelCount,
				},
			},
		}
		bagUpdate.ItemOps = append(bagUpdate.ItemOps, itemOp)
	}
	return realDelCount
}
