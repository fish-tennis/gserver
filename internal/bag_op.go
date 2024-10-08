package internal

// 背包通用操作接口
type BagOp interface {
	// 背包容量
	GetCapacity() int32

	// 获取物品数量
	GetItemCount(itemCfgId int32) int32

	// 添加物品,返回实际添加数量
	AddItem(itemCfgId, addCount int32) int32

	// 删除指定数量物品,返回实际删除数量
	DelItem(itemCfgId, delCount int32) int32
}

// 有唯一id的对象
type Uniquely interface {
	GetUniqueId() int64
}

// 限时类对象
type TimeLimited interface {
	// 超时时间戳
	GetTimeout() int32
}
