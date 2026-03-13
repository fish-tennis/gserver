package game

import (
	"log/slog"
	"reflect"

	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// 以配置id作为key的不可叠加的容器
type CfgContainer[E internal.CfgData] struct {
	*gentity.MapData[int32, E] `db:""`
	Bags                       *Bags
	containerType              pb.ContainerType
	ElemCtor                   func(arg *pb.AddElemArg) E // 元素的构造接口
	//timeoutCheckList           []*timeoutCheckData        // 限时类物品超时检查列表(排序的)
}

func NewCfgContainer[E internal.CfgData](bags *Bags, bagType pb.ContainerType, elemCtor func(arg *pb.AddElemArg) E) *CfgContainer[E] {
	return &CfgContainer[E]{
		MapData:       gentity.NewMapData[int32, E](),
		Bags:          bags,
		containerType: bagType,
		ElemCtor:      elemCtor,
	}
}

func (b *CfgContainer[E]) GetCapacity() int32 {
	return 100
}

func (b *CfgContainer[E]) GetElemCount(itemCfgId int32) int32 {
	if b.Contains(itemCfgId) {
		return 1
	}
	return 0
}

func (b *CfgContainer[E]) GetElem(itemCfgId int32) E {
	return b.Data[itemCfgId]
}

func (b *CfgContainer[E]) AddCfgElem(e E) int32 {
	if len(b.Data) >= int(b.GetCapacity()) {
		slog.Debug("BagFull", "cfgId", e.GetCfgId())
		return 0
	}
	if _, ok := b.Data[e.GetCfgId()]; !ok {
		b.Set(e.GetCfgId(), e)
		//// 加入超时检测列表
		//if timeoutItem, ok := any(e).(internal.TimeLimited); ok && timeoutItem.GetTimeout() > 0 {
		//	b.addToTimeoutList(e.GetUniqueId(), timeoutItem.GetTimeout())
		//}
		slog.Debug("AddCfgElem", "cfgId", e.GetCfgId())
		return e.GetCfgId()
	}
	return 0
}

func (b *CfgContainer[E]) AddElem(arg *pb.AddElemArg, bagUpdate *pb.ElemContainerUpdate) int32 {
	if arg.GetNum() <= 0 {
		return 0
	}
	//itemCfg := cfg.ItemCfgs.GetCfg(arg.GetId())
	//if itemCfg == nil {
	//	return 0
	//}
	if len(b.Data) >= int(b.GetCapacity()) {
		slog.Debug("BagFull", "cfgId", arg.GetCfgId(), "addCount", arg.GetNum())
		return int32(0)
	}
	cfgItem := b.ElemCtor(arg)
	//// 限时道具
	//timeout := int32(0)
	//if arg.GetTimeType() > 0 {
	//	// 可以在添加物品的时候,附加限时属性
	//	timeout = util.GetTimeoutTimestamp(arg.GetTimeType(), arg.GetTimeout(), b.Bags.GetPlayer().GetTimerEntries().Now())
	//} else if itemCfg.GetItemType() > 0 {
	//	// 也可以在物品配置表里配置限时属性
	//	timeout = util.GetTimeoutTimestamp(itemCfg.GetTimeType(), itemCfg.GetTimeout(), b.Bags.GetPlayer().GetTimerEntries().Now())
	//}
	//if timeout > 0 {
	//	// NOTE:假设固定字段是Timeout
	//	reflect.ValueOf(cfgItem).Elem().FieldByName("Timeout").SetInt(int64(timeout))
	//}
	newId := b.AddCfgElem(cfgItem)
	if bagUpdate != nil && newId > 0 {
		itemOp := &pb.ElemOp{
			ContainerType: b.containerType,
			OpType:        pb.ElemOpType_ElemOpType_Add,
		}
		switch realItem := any(cfgItem).(type) {
		case proto.Message:
			itemOp.ElemData, _ = anypb.New(realItem)
		default:
			// TODO: 使用类似ItemCtor的方式,传一个自定义的序列化接口进来
			slog.Error("AddElemErr", "containerType", b.containerType, "itemType", reflect.TypeOf(cfgItem))
		}
		if itemOp.ElemData != nil {
			bagUpdate.ElemOps = append(bagUpdate.ElemOps, itemOp)
		}
	}
	return 1
}

func (b *CfgContainer[E]) AddElemAndSyncData(arg *pb.AddElemArg) int32 {
	bagUpdate := &pb.ElemContainerUpdate{}
	num := b.AddElem(arg, bagUpdate)
	if len(bagUpdate.ElemOps) > 0 {
		b.Bags.GetPlayer().Send(bagUpdate)
	}
	return num
}

func (b *CfgContainer[E]) DelElem(arg *pb.DelElemArg, bagUpdate *pb.ElemContainerUpdate) int32 {
	if arg.GetNum() <= 0 {
		return 0
	}
	if b.Contains(arg.GetCfgId()) {
		b.Delete(arg.GetCfgId())
		return 1
	}
	return 0
}

func (b *CfgContainer[E]) UpdateElem(elem E, bagUpdate *pb.ElemContainerUpdate) {
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
		// TODO: 使用类似ItemCtor的方式,传一个自定义的序列化接口进来
		slog.Error("UpdateElemErr", "containerType", b.containerType, "itemType", reflect.TypeOf(elem))
	}
	if itemOp.ElemData != nil {
		bagUpdate.ElemOps = append(bagUpdate.ElemOps, itemOp)
	}
	if syncUpdateData {
		b.Bags.GetPlayer().Send(bagUpdate)
	}
}

//// 加载数据后,把限时类物品加入超时检查列表
//func (b *CfgContainer[E]) initTimeoutList() {
//	b.timeoutCheckList = nil
//	b.Range(func(uniqueId int64, e E) bool {
//		if timeoutItem, ok := any(e).(internal.TimeLimited); ok && timeoutItem.GetTimeout() > 0 {
//			b.addToTimeoutList(e.GetUniqueId(), timeoutItem.GetTimeout())
//		}
//		return true
//	})
//}
//
//// 加到限时检测列表
//func (b *CfgContainer[E]) addToTimeoutList(uniqueId int64, timeout int32) {
//	b.timeoutCheckList = append(b.timeoutCheckList, &timeoutCheckData{
//		uniqueId: uniqueId,
//		timeout:  timeout,
//	})
//	// 从大到小排序
//	slices.SortFunc(b.timeoutCheckList, func(a, b *timeoutCheckData) int {
//		if a.timeout < b.timeout {
//			return 1
//		}
//		if a.timeout > b.timeout {
//			return -1
//		}
//		return 0
//	})
//	slog.Debug("addToTimeoutList", "uniqueId", uniqueId, "timeout", timeout, "list", b.timeoutCheckList)
//}
//
//// 移出限时检测列表
//func (b *CfgContainer[E]) removeFromTimeoutList(uniqueId int64) {
//	for i, v := range b.timeoutCheckList {
//		if v.uniqueId == uniqueId {
//			removed := b.timeoutCheckList[i]
//			b.timeoutCheckList = append(b.timeoutCheckList[:i], b.timeoutCheckList[i+1:]...)
//			slog.Debug("removeFromTimeoutList", "uniqueId", uniqueId, "i", i, "removed", removed)
//			return
//		}
//	}
//}
//
//// 检查限时物品超时
//func (b *CfgContainer[E]) checkTimeout(now int32, bagUpdate *pb.ElemContainerUpdate) {
//	for i := len(b.timeoutCheckList) - 1; i >= 0; i-- {
//		if b.timeoutCheckList[i].timeout > now {
//			// 最后一个还没过期,直接返回,因为排过序了
//			return
//		}
//		uniqueId := b.timeoutCheckList[i].uniqueId
//		slog.Debug("checkTimeout", "uniqueId", uniqueId, "i", i, "v", b.timeoutCheckList[i])
//		if b.DelUniqueItem(uniqueId, bagUpdate) == 0 {
//			b.timeoutCheckList = append(b.timeoutCheckList[:i], b.timeoutCheckList[i+1:]...)
//			slog.Error("timeoutErr", "uniqueId", uniqueId, "len", len(b.timeoutCheckList))
//			continue
//		}
//		slog.Debug("timeout", "uniqueId", uniqueId, "len", len(b.timeoutCheckList))
//	}
//}
