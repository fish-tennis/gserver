package entity

// 实体接口
type Entity interface {
	// 查找某个组件
	GetComponent(componentName string) Component

	// 组件列表
	GetComponents() []Component
}

// 实体组件接口
type Component interface {
	//GetId() int

	// 组件名
	GetName() string
	GetNameLower() string

	// 所属的实体
	GetEntity() Entity
}

// 保存数据接口
type Saveable interface {
	// 序列化
	Serialize(forCache bool) interface{}
	// 反序列化
	Deserialize(bytes []byte) error
	// 需要保存的数据是否修改了
	IsDirty() bool
	// 设置数据修改标记
	SetDirty()
	// 重置标记
	ResetDirty()
}

// 事件接口
type EventReceiver interface {
	OnEvent(event interface{})
}
