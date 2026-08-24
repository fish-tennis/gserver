package game

import (
	"log/slog"
	"reflect"

	"math"

	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// 以配置id作为key的可叠加容器,元素同时具有配置id和数量属性(如宝石背包)
// E 必须同时实现 CfgData(GetId) 和 CountItem(GetCount)
type CfgCountContainer[E internal.CfgData] struct {
	*gentity.MapData[int32, E] `db:""`
	Bags                       *Bags
	containerType              pb.ContainerType
	ElemCtor                   func(arg *pb.AddElemArg) E
}

func NewCfgCountContainer[E internal.CfgData](bags *Bags, bagType pb.ContainerType, elemCtor func(arg *pb.AddElemArg) E) *CfgCountContainer[E] {
	return &CfgCountContainer[E]{
		MapData:       gentity.NewMapData[int32, E](),
		Bags:          bags,
		containerType: bagType,
		ElemCtor:      elemCtor,
	}
}

// 容量
// 实际项目可改为读取配置或扩展背包变量
func (b *CfgCountContainer[E]) GetCapacity() int32 {
	return internal.DefaultContainerCapacity
}

// 返回元素的 GetCount() 值
func (b *CfgCountContainer[E]) GetElemCount(itemCfgId int32) int32 {
	if elem, ok := b.Data[itemCfgId]; ok {
		if countItem, ok := any(elem).(internal.CountItem); ok {
			return countItem.GetCount()
		}
	}
	return 0
}

func (b *CfgCountContainer[E]) GetElem(itemCfgId int32) E {
	return b.Data[itemCfgId]
}

// 设置元素的 Count 字段,通过 reflect 修改 protobuf 生成的结构体
func setElemCount[E internal.CfgData](elem E, count int32) {
	reflect.ValueOf(elem).Elem().FieldByName("Count").SetInt(int64(count))
}

func (b *CfgCountContainer[E]) AddElem(arg *pb.AddElemArg, bagUpdate *pb.ElemContainerUpdate) int32 {
	addCount := arg.GetNum()
	if addCount <= 0 {
		return 0
	}
	if existing, ok := b.Data[arg.GetCfgId()]; ok {
		// 元素已存在,叠加数量
		curCount := int32(0)
		if countItem, ok := any(existing).(internal.CountItem); ok {
			curCount = countItem.GetCount()
		}
		// 检查 int32 溢出
		if int64(curCount)+int64(addCount) > math.MaxInt32 {
			addCount = math.MaxInt32 - curCount
		}
		newCount := curCount + addCount
		setElemCount(existing, newCount)
		// 触发 MapData 的修改标记
		b.Set(arg.GetCfgId(), existing)
		slog.Debug("AddElemStack", "cfgId", arg.GetCfgId(), "curCount", curCount, "addCount", addCount, "newCount", newCount)
		if bagUpdate != nil && addCount > 0 {
			itemOp := &pb.ElemOp{
				ContainerType: b.containerType,
				OpType:        pb.ElemOpType_ElemOpType_Update,
			}
			switch realItem := any(existing).(type) {
			case proto.Message:
				itemOp.ElemData, _ = anypb.New(realItem)
			default:
				slog.Error("AddElemStackErr", "containerType", b.containerType, "itemType", reflect.TypeOf(existing))
			}
			if itemOp.ElemData != nil {
				bagUpdate.ElemOps = append(bagUpdate.ElemOps, itemOp)
			}
		}
		return addCount
	}
	// 元素不存在,创建新元素
	if len(b.Data) >= int(b.GetCapacity()) {
		slog.Debug("BagFull", "cfgId", arg.GetCfgId(), "addCount", addCount)
		return 0
	}
	cfgItem := b.ElemCtor(arg)
	newId := arg.GetCfgId()
	b.Set(newId, cfgItem)
	slog.Debug("AddElemNew", "cfgId", newId)
	if bagUpdate != nil {
		itemOp := &pb.ElemOp{
			ContainerType: b.containerType,
			OpType:        pb.ElemOpType_ElemOpType_Add,
		}
		switch realItem := any(cfgItem).(type) {
		case proto.Message:
			itemOp.ElemData, _ = anypb.New(realItem)
		default:
			slog.Error("AddElemErr", "containerType", b.containerType, "itemType", reflect.TypeOf(cfgItem))
		}
		if itemOp.ElemData != nil {
			bagUpdate.ElemOps = append(bagUpdate.ElemOps, itemOp)
		}
	}
	return addCount
}

func (b *CfgCountContainer[E]) AddElemAndSyncData(arg *pb.AddElemArg) int32 {
	bagUpdate := &pb.ElemContainerUpdate{}
	num := b.AddElem(arg, bagUpdate)
	if len(bagUpdate.ElemOps) > 0 {
		b.Bags.GetPlayer().Send(bagUpdate)
	}
	return num
}

func (b *CfgCountContainer[E]) DelElem(arg *pb.DelElemArg, bagUpdate *pb.ElemContainerUpdate) int32 {
	delCount := arg.GetNum()
	if delCount <= 0 {
		return 0
	}
	existing, ok := b.Data[arg.GetCfgId()]
	if !ok {
		return 0
	}
	curCount := int32(0)
	if countItem, ok := any(existing).(internal.CountItem); ok {
		curCount = countItem.GetCount()
	}
	realDelCount := delCount
	if delCount >= curCount {
		// 数量不足或刚好,删除整个元素
		b.Delete(arg.GetCfgId())
		realDelCount = curCount
		slog.Debug("DelElemFull", "cfgId", arg.GetCfgId(), "curCount", curCount, "delCount", delCount)
		if bagUpdate != nil {
			itemOp := &pb.ElemOp{
				ContainerType: b.containerType,
				OpType:        pb.ElemOpType_ElemOpType_Delete,
			}
			switch realItem := any(existing).(type) {
			case proto.Message:
				itemOp.ElemData, _ = anypb.New(realItem)
			default:
				slog.Error("DelElemErr", "containerType", b.containerType, "itemType", reflect.TypeOf(existing))
			}
			if itemOp.ElemData != nil {
				bagUpdate.ElemOps = append(bagUpdate.ElemOps, itemOp)
			}
		}
	} else {
		// 部分删除,减少 Count
		newCount := curCount - delCount
		setElemCount(existing, newCount)
		b.Set(arg.GetCfgId(), existing)
		slog.Debug("DelElemPartial", "cfgId", arg.GetCfgId(), "curCount", curCount, "delCount", delCount, "newCount", newCount)
		if bagUpdate != nil {
			itemOp := &pb.ElemOp{
				ContainerType: b.containerType,
				OpType:        pb.ElemOpType_ElemOpType_Update,
			}
			switch realItem := any(existing).(type) {
			case proto.Message:
				itemOp.ElemData, _ = anypb.New(realItem)
			default:
				slog.Error("DelElemPartialErr", "containerType", b.containerType, "itemType", reflect.TypeOf(existing))
			}
			if itemOp.ElemData != nil {
				bagUpdate.ElemOps = append(bagUpdate.ElemOps, itemOp)
			}
		}
	}
	return realDelCount
}

func (b *CfgCountContainer[E]) UpdateElem(elem E, bagUpdate *pb.ElemContainerUpdate) {
	b.Set(elem.GetCfgId(), elem)
	syncUpdateData := false
	if bagUpdate == nil {
		bagUpdate = &pb.ElemContainerUpdate{}
		syncUpdateData = true
	}
	itemOp := &pb.ElemOp{
		ContainerType: b.containerType,
		OpType:        pb.ElemOpType_ElemOpType_Update,
	}
	switch realItem := any(elem).(type) {
	case proto.Message:
		itemOp.ElemData, _ = anypb.New(realItem)
	default:
		slog.Error("UpdateElemErr", "containerType", b.containerType, "itemType", reflect.TypeOf(elem))
	}
	if itemOp.ElemData != nil {
		bagUpdate.ElemOps = append(bagUpdate.ElemOps, itemOp)
	}
	if syncUpdateData {
		b.Bags.GetPlayer().Send(bagUpdate)
	}
}
