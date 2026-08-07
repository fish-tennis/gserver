package game

import (
	"log/slog"
	"reflect"
	"slices"

	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type timeoutCheckData struct {
	uniqueId int64 // 物品唯一id
	timeout  int32 // 超时时间戳(秒)
}

// 通用的不可叠加的物品容器(如装备背包或限时道具背包)
type UniqueContainer[E internal.Uniquely] struct {
	*gentity.MapData[int64, E] `db:""`
	Bags                       *Bags
	containerType              pb.ContainerType
	ElemCtor                   func(arg *pb.AddElemArg) E // 元素的构造接口
	timeoutCheckList           []*timeoutCheckData        // 限时类物品超时检查列表(排序的)
}

func NewBagUnique[E internal.Uniquely](bags *Bags, bagType pb.ContainerType, elemCtor func(arg *pb.AddElemArg) E) *UniqueContainer[E] {
	return &UniqueContainer[E]{
		MapData:       gentity.NewMapData[int64, E](),
		Bags:          bags,
		containerType: bagType,
		ElemCtor:      elemCtor,
	}
}

// 容量
// 不可叠加物品的容量,实际项目可改为读取配置或扩展背包变量
func (b *UniqueContainer[E]) GetCapacity() int32 {
	return internal.DefaultContainerCapacity
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
		slog.Debug("AddUniqueItem", "cfgId", e.GetCfgId(), "uniqueId", e.GetUniqueId())
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
	// 限制单次添加数量,防止客户端传入超大值导致CPU/内存耗尽
	if addCount > internal.MaxBatchAddUniqueElemCount {
		addCount = internal.MaxBatchAddUniqueElemCount
	}
	itemCfg := cfg.ItemCfgs.GetCfg(arg.GetCfgId())
	if itemCfg == nil {
		return 0
	}
	realAdded := int32(0)
	for i := 0; i < int(addCount); i++ {
		if len(b.Data) >= int(b.GetCapacity()) {
			slog.Debug("BagFull", "cfgId", arg.GetCfgId(), "addCount", addCount, "realAddCount", realAdded)
			return realAdded
		}
		uniqueItem := b.ElemCtor(arg)
		// 限时道具
		timeout := int32(0)
		if arg.GetTimeType() > 0 {
			// 可以在添加物品的时候,附加限时属性
			timeout = util.GetTimeoutTimestamp(arg.GetTimeType(), arg.GetTimeout(), b.Bags.GetPlayer().GetTimerEntries().Now())
		} else if itemCfg.GetTimeType() > 0 {
			// 也可以在物品配置表里配置限时属性
			timeout = util.GetTimeoutTimestamp(itemCfg.GetTimeType(), itemCfg.GetTimeout(), b.Bags.GetPlayer().GetTimerEntries().Now())
		}
		if timeout > 0 {
			// NOTE:假设固定字段是Timeout
			timeoutField := reflect.ValueOf(uniqueItem).Elem().FieldByName("Timeout")
			if timeoutField.IsValid() && timeoutField.CanSet() {
				timeoutField.SetInt(int64(timeout))
			} else {
				// 反射设置失败,物品仍会添加但 Timeout 为 0,限时道具将变成永久道具
				// 这是编程错误(物品结构体缺少 Timeout 字段),需要修复物品定义
				slog.Error("AddElemErr TimeoutFieldNotFound: timed item added as permanent, fix struct definition",
					"containerType", b.containerType,
					"itemType", reflect.TypeOf(uniqueItem),
					"cfgId", arg.GetCfgId(),
					"expectedTimeout", timeout)
			}
		}
		newUniqueId := b.AddUniqueItem(uniqueItem)
		if newUniqueId > 0 {
			realAdded++
			if bagUpdate != nil {
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
	}
	return realAdded
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

// 加到限时检测列表(已按 timeout 降序排列,用二分查找插入位置)
func (b *UniqueContainer[E]) addToTimeoutList(uniqueId int64, timeout int32) {
	entry := &timeoutCheckData{
		uniqueId: uniqueId,
		timeout:  timeout,
	}
	// 二分查找插入位置:列表按 timeout 降序,找到第一个 timeout <= entry 的位置
	insertIdx, _ := slices.BinarySearchFunc(b.timeoutCheckList, entry, func(a, c *timeoutCheckData) int {
		if a.timeout > c.timeout {
			return -1 // a 应在 c 前面
		}
		return 1 // a 应在 c 后面(或相等)
	})
	b.timeoutCheckList = slices.Insert(b.timeoutCheckList, insertIdx, entry)
	slog.Debug("addToTimeoutList", "uniqueId", uniqueId, "timeout", timeout, "insertIdx", insertIdx)
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
// 列表按 timeout 降序排列(大→小),尾部是最早过期的
// 从尾部向前收集所有过期项,统一截断列表,再逐个从 Data 中删除
// 避免每次 DelUniqueItem 都触发 removeFromTimeoutList 的 O(n) 扫描,总复杂度从 O(n²) 降为 O(n)
func (b *UniqueContainer[E]) checkTimeout(now int32, bagUpdate *pb.ElemContainerUpdate) {
	if len(b.timeoutCheckList) == 0 {
		return
	}
	// 从尾部向前找第一个未过期的位置
	expiredEnd := len(b.timeoutCheckList)
	for i := len(b.timeoutCheckList) - 1; i >= 0; i-- {
		if b.timeoutCheckList[i].timeout > now {
			break
		}
		expiredEnd = i
	}
	if expiredEnd >= len(b.timeoutCheckList) {
		return // 没有过期项
	}
	// 收集过期项并统一截断列表
	expiredItems := b.timeoutCheckList[expiredEnd:]
	b.timeoutCheckList = b.timeoutCheckList[:expiredEnd]
	for _, item := range expiredItems {
		uniqueId := item.uniqueId
		// 直接从 Data 中删除,不再调用 removeFromTimeoutList(列表已截断)
		if e, ok := b.Data[uniqueId]; ok {
			b.Delete(uniqueId)
			slog.Debug("checkTimeout", "uniqueId", uniqueId)
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
		} else {
			slog.Error("checkTimeout item not in Data", "uniqueId", uniqueId)
		}
	}
}
