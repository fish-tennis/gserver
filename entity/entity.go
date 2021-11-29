package entity

// 实体接口
type Entity interface {
	// 查找某个组件
	GetComponent(componentId int) Component

	// 组件列表
	GetComponents() []Component
}

// 实体组件接口
type Component interface {

	GetId() int

	GetName() string

	// 所属的实体
	GetEntity() Entity
}

// 保存数据接口
type Saveable interface {
	// 保存数据,保存成功后,重置dirty
	Save() error
	// 需要保存的数据是否修改了
	IsDirty() bool
	// 设置数据修改标记
	SetDirty()
	// 重置标记
	ResetDirty()
}
