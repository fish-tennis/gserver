package internal

import "github.com/fish-tennis/gserver/pb"

// 背包通用操作接口
type Bag interface {
	// 背包容量
	GetCapacity() int32

	// 获取物品数量
	GetItemCount(itemCfgId int32) int32

	// 添加物品,返回实际添加数量
	AddItem(arg *pb.AddItemArg, bagUpdate *pb.BagUpdate) int32

	// 删除指定数量物品,返回实际删除数量
	DelItem(arg *pb.DelItemArg, bagUpdate *pb.BagUpdate) int32
}

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
