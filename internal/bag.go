package internal

import "github.com/fish-tennis/gserver/pb"

// 容器通用操作接口(例如背包就是一种容器)
type ElemContainer interface {
	// 容量
	GetCapacity() int32

	// 获取元素数量
	GetElemCount(elemCfgId int32) int32

	// 添加元素,返回实际添加数量
	AddElem(arg *pb.AddElemArg, containerUpdate *pb.ElemContainerUpdate) int32

	// 删除指定数量元素,返回实际删除数量
	DelElem(arg *pb.DelElemArg, containerUpdate *pb.ElemContainerUpdate) int32
}

// 默认容器容量,实际项目应根据配置或变量动态调整(如扩展背包)
const DefaultContainerCapacity int32 = 10000

// 有唯一id的对象
type Uniquely interface {
	// 配置id
	GetCfgId() int32
	// uuid
	GetUniqueId() int64
}

// 限时类对象
type TimeLimited interface {
	// 超时时间戳
	GetTimeout() int32
	//SetTimeout(timeout int32)
}

// 带数量的物品
type CountItem interface {
	// 数量
	GetCount() int32
}
