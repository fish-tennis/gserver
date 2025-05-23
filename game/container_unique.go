package game

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"log/slog"
	"reflect"
	"slices"
)

type timeoutCheckData struct {
	uniqueId int64 // 物品唯一id
	timeout  int32 // 超时时间戳(秒)
}

// 通用的不可叠加的物品容器(如装备背包或限时道具背包)
type UniqueContainer[E internal.Uniquely] struct {
	*gentity.MapData[int64, E] `db:""`
	containerType              pb.ContainerType
	ElemCtor                   func(arg *pb.AddElemArg) E // 元素的构造接口
	timeoutCheckList           []*timeoutCheckData        // 限时类物品超时检查列表(排序的)
}

func NewBagUnique[E internal.Uniquely](bagType pb.ContainerType, elemCtor func(arg *pb.AddElemArg) E) *UniqueContainer[E] {
	return &UniqueContainer[E]{
		MapData:       gentity.NewMapData[int64, E](),
		containerType: bagType,
		ElemCtor:      elemCtor,
	}
}

func (b *UniqueContainer[E]) GetCapacity() int32 {
	return 100
}

func (b *UniqueContainer[E]) GetElemCount(itemCfgId int32) int32 {
	itemCount := int32(0)
	for _, item := range b.Data {
		if item.GetCfgId() == itemCfgId {
			itemCount++
		}
	}
	return itemCount
}

func (b *UniqueContainer[E]) AddUniqueItem(e E) int64 {
	if len(b.Data) >= int(b.GetCapacity()) {
		slog.Debug("BagFull", "cfgId", e.GetCfgId(), "uniqueId", e.GetUniqueId())
		return 0
	}
	if _, ok := b.Data[e.GetUniqueId()]; !ok {
		b.Set(e.GetUniqueId(), e)
		// 加入超时检测列表
		if timeoutItem, ok := any(e).(internal.TimeLimited); ok && timeoutItem.GetTimeout() > 0 {
			b.addToTimeoutList(e.GetUniqueId(), timeoutItem.GetTimeout())
		}
		logger.Debug("AddUniqueItem CfgId:%v UniqueId:%v", e.GetCfgId(), e.GetUniqueId())
		return e.GetUniqueId()
	}
	return 0
}

func (b *UniqueContainer[E]) DelUniqueItem(uniqueId int64, bagUpdate *pb.ElemContainerUpdate) int32 {
	if e, ok := b.Data[uniqueId]; ok {
		b.Delete(uniqueId)
		// 移除超时检测列表
		if _, ok := any(e).(internal.TimeLimited); ok {
			b.removeFromTimeoutList(e.GetUniqueId())
		}
		slog.Debug("DelUniqueItem", "uniqueId", uniqueId)
		if bagUpdate != nil {
			itemOp := &pb.ElemOp{
				ContainerType: b.containerType,
				OpType:        pb.ElemOpType_ElemOpType_Delete,
			}
			itemOp.ElemData, _ = anypb.New(&pb.UniqueId{
				Id: e.GetUniqueId(),
			})
			bagUpdate.ElemOps = append(bagUpdate.ElemOps, itemOp)
		}
		return 1
	}
	return 0
}

func (b *UniqueContainer[E]) AddElem(arg *pb.AddElemArg, bagUpdate *pb.ElemContainerUpdate) int32 {
	addCount := arg.GetNum()
	if addCount <= 0 {
		return 0
	}
	itemCfg := cfg.GetItemCfgMgr().GetItemCfg(arg.GetCfgId())
	if itemCfg == nil {
		return 0
	}
	for i := 0; i < int(addCount); i++ {
		if len(b.Data) >= int(b.GetCapacity()) {
			slog.Debug("BagFull", "cfgId", arg.GetCfgId(), "addCount", addCount, "realAddCount", i)
			return int32(i)
		}
		uniqueItem := b.ElemCtor(arg)
		// 限时道具
		timeout := int32(0)
		if arg.GetTimeType() > 0 {
			// 可以在添加物品的时候,附加限时属性
			timeout = util.GetTimeoutTimestamp(arg.GetTimeType(), arg.GetTimeout())
		} else if itemCfg.GetItemType() > 0 {
			// 也可以在物品配置表里配置限时属性
			timeout = util.GetTimeoutTimestamp(itemCfg.GetTimeType(), itemCfg.GetTimeout())
		}
		if timeout > 0 {
			// NOTE:假设固定字段是Timeout
			reflect.ValueOf(uniqueItem).Elem().FieldByName("Timeout").SetInt(int64(timeout))
		}
		newUniqueId := b.AddUniqueItem(uniqueItem)
		if bagUpdate != nil && newUniqueId > 0 {
			itemOp := &pb.ElemOp{
				ContainerType: b.containerType,
				OpType:        pb.ElemOpType_ElemOpType_Add,
			}
			switch realItem := any(uniqueItem).(type) {
			case proto.Message:
				itemOp.ElemData, _ = anypb.New(realItem)
			default:
				// TODO: 使用类似ItemCtor的方式,传一个自定义的序列化接口进来
				slog.Error("AddItemErr", "containerType", b.containerType, "itemType", reflect.TypeOf(uniqueItem))
			}
			if itemOp.ElemData != nil {
				bagUpdate.ElemOps = append(bagUpdate.ElemOps, itemOp)
			}
		}
	}
	return addCount
}

func (b *UniqueContainer[E]) DelElem(arg *pb.DelElemArg, bagUpdate *pb.ElemContainerUpdate) int32 {
	realDelCount := int32(0)
	// 删除指定物品
	if arg.GetUniqueId() > 0 {
		return b.DelUniqueItem(arg.GetUniqueId(), bagUpdate)
	}
	if arg.GetNum() <= 0 {
		return 0
	}
	for _, e := range b.Data {
		if e.GetCfgId() == arg.GetCfgId() {
			b.Delete(e.GetUniqueId())
			realDelCount++
			// 加入超时检测列表
			if _, ok := any(e).(internal.TimeLimited); ok {
				b.removeFromTimeoutList(e.GetUniqueId())
			}
			if bagUpdate != nil {
				itemOp := &pb.ElemOp{
					ContainerType: b.containerType,
					OpType:        pb.ElemOpType_ElemOpType_Delete,
				}
				itemOp.ElemData, _ = anypb.New(&pb.UniqueId{
					Id: e.GetUniqueId(),
				})
				bagUpdate.ElemOps = append(bagUpdate.ElemOps, itemOp)
			}
			slog.Debug("DelElem", "cfgId", arg.GetCfgId(), "uniqueId", e.GetUniqueId())
			if realDelCount >= arg.GetNum() {
				break
			}
		}
	}
	return realDelCount
}

// 加载数据后,把限时类物品加入超时检查列表
func (b *UniqueContainer[E]) initTimeoutList() {
	b.timeoutCheckList = nil
	b.Range(func(uniqueId int64, e E) bool {
		if timeoutItem, ok := any(e).(internal.TimeLimited); ok && timeoutItem.GetTimeout() > 0 {
			b.addToTimeoutList(e.GetUniqueId(), timeoutItem.GetTimeout())
		}
		return true
	})
}

// 加到限时检测列表
func (b *UniqueContainer[E]) addToTimeoutList(uniqueId int64, timeout int32) {
	b.timeoutCheckList = append(b.timeoutCheckList, &timeoutCheckData{
		uniqueId: uniqueId,
		timeout:  timeout,
	})
	// 从大到小排序
	slices.SortFunc(b.timeoutCheckList, func(a, b *timeoutCheckData) int {
		if a.timeout < b.timeout {
			return 1
		}
		if a.timeout > b.timeout {
			return -1
		}
		return 0
	})
	slog.Debug("addToTimeoutList", "uniqueId", uniqueId, "timeout", timeout, "list", b.timeoutCheckList)
}

// 移出限时检测列表
func (b *UniqueContainer[E]) removeFromTimeoutList(uniqueId int64) {
	for i, v := range b.timeoutCheckList {
		if v.uniqueId == uniqueId {
			removed := b.timeoutCheckList[i]
			b.timeoutCheckList = append(b.timeoutCheckList[:i], b.timeoutCheckList[i+1:]...)
			slog.Debug("removeFromTimeoutList", "uniqueId", uniqueId, "i", i, "removed", removed)
			return
		}
	}
}

// 检查限时物品超时
func (b *UniqueContainer[E]) checkTimeout(now int32, bagUpdate *pb.ElemContainerUpdate) {
	for i := len(b.timeoutCheckList) - 1; i >= 0; i-- {
		if b.timeoutCheckList[i].timeout > now {
			// 最后一个还没过期,直接返回,因为排过序了
			return
		}
		uniqueId := b.timeoutCheckList[i].uniqueId
		slog.Debug("checkTimeout", "uniqueId", uniqueId, "i", i, "v", b.timeoutCheckList[i])
		if b.DelUniqueItem(uniqueId, bagUpdate) == 0 {
			b.timeoutCheckList = append(b.timeoutCheckList[:i], b.timeoutCheckList[i+1:]...)
			slog.Error("timeoutErr", "uniqueId", uniqueId, "len", len(b.timeoutCheckList))
			continue
		}
		slog.Debug("timeout", "uniqueId", uniqueId, "len", len(b.timeoutCheckList))
	}
}
