package internal

// 实体接口
type Entity interface {
	// 查找某个组件
	GetComponent(componentName string) Component

	// 组件列表
	GetComponents() []Component
}

// 实体组件接口
type Component interface {
	// 组件名
	GetName() string
	GetNameLower() string

	// 所属的实体
	GetEntity() Entity
}

// 事件接口
type EventReceiver interface {
	OnEvent(event interface{})
}