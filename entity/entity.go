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

	Load() error
	Save() error
}
