package game

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/types/known/anypb"
	"math"
)

// 有数量的容器(如常见的道具背包,每个格子只需要记录一个配置id和数量即可)
type CountContainer struct {
	*gentity.MapData[int32, int32] `db:""`
	containerType                  pb.ContainerType
}

func NewBagCountItem() *CountContainer {
	bag := &CountContainer{
		MapData:       gentity.NewMapData[int32, int32](),
		containerType: pb.ContainerType_ContainerType_CountItem,
	}
	return bag
}

// 容量
func (b *CountContainer) GetCapacity() int32 {
	// 实际项目读取配置或者是个变量,比如可以扩展背包容量
	return 100
}

func (b *CountContainer) GetElemCount(itemCfgId int32) int32 {
	return b.Data[itemCfgId]
}

func (b *CountContainer) AddElem(arg *pb.AddElemArg, containerUpdate *pb.ElemContainerUpdate) int32 {
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
	logger.Debug("AddElem cfgId:%v curCount:%v addCount:%v", arg.GetCfgId(), curCount, addCount)
	if containerUpdate != nil && addCount > 0 {
		itemOp := &pb.ElemOp{
			ContainerType: b.containerType,
			OpType:        pb.ElemOpType_ElemOpType_Add,
		}
		itemOp.ElemData, _ = anypb.New(&pb.ElemNum{
			CfgId: arg.GetCfgId(),
			Num:   addCount,
		})
		containerUpdate.ElemOps = append(containerUpdate.ElemOps, itemOp)
	}
	return addCount
}

func (b *CountContainer) DelElem(arg *pb.DelElemArg, bagUpdate *pb.ElemContainerUpdate) int32 {
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
		logger.Debug("DelElem cfgId:%v delCount:%v/%v", arg.GetCfgId(), curCount, delCount)
		realDelCount = curCount
	} else {
		b.Set(arg.GetCfgId(), curCount-delCount)
		logger.Debug("DelElem cfgId:%v delCount:%v", arg.GetCfgId(), delCount)
	}
	if bagUpdate != nil {
		itemOp := &pb.ElemOp{
			ContainerType: b.containerType,
			OpType:        pb.ElemOpType_ElemOpType_Delete,
		}
		itemOp.ElemData, _ = anypb.New(&pb.ElemNum{
			CfgId: arg.GetCfgId(),
			Num:   realDelCount,
		})
		bagUpdate.ElemOps = append(bagUpdate.ElemOps, itemOp)
	}
	return realDelCount
}
