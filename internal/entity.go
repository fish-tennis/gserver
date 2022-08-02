package internal

import (
	"fmt"
	"github.com/fish-tennis/gserver/logger"
	"strings"
)

// 实体接口
type Entity interface {
	// 唯一id
	GetId() int64

	// 查找某个组件
	GetComponentByName(componentName string) Component

	// 遍历组件
	RangeComponent(fun func(component Component) bool)
}

// 实体组件接口
type Component interface {
	// 组件名
	GetName() string
	GetNameLower() string

	// 所属的实体
	GetEntity() Entity
	SetEntity(entity Entity)
}

// 事件接口
type EventReceiver interface {
	OnEvent(event interface{})
}

type BaseEntity struct {
	// 组件表
	components []Component
}

// 获取组件
func (this *BaseEntity) GetComponentByName(componentName string) Component {
	for _,v := range this.components {
		if v.GetName() == componentName {
			return v
		}
	}
	return nil
}

func (this *BaseEntity) GetComponentByIndex(componentIndex int) Component {
	return this.components[componentIndex]
}

// 组件列表
func (this *BaseEntity) GetComponents() []Component {
	return this.components
}

func (this *BaseEntity) RangeComponent(fun func(component Component) bool) {
	for _,v := range this.components {
		if !fun(v) {
			return
		}
	}
}

func (this *BaseEntity) AddComponent(component Component, sourceData interface{}) {
	if len(component.GetName()) == 0 {
		logger.Error("Component name empty")
	}
	if sourceData != nil {
		if saveable, ok := component.(Saveable); ok {
			LoadSaveable(saveable, sourceData)
		}
		if compositeSaveable, ok := component.(CompositeSaveable); ok {
			LoadCompositeSaveable(compositeSaveable, sourceData)
		}
	}
	this.components = append(this.components, component)
}

func (this *BaseEntity) SaveCache() error {
	for _, component := range this.components {
		SaveDirtyCache(component)
	}
	return nil
}

type BaseComponent struct {
	entity Entity
	// 组件名
	name string
}

// 组件名
func (this *BaseComponent) GetName() string {
	return this.name
}

func (this *BaseComponent) GetNameLower() string {
	return strings.ToLower(this.name)
}

func (this *BaseComponent) GetEntity() Entity {
	return this.entity
}

func (this *BaseComponent) SetEntity(entity Entity) {
	this.entity = entity
}


type DataComponent struct {
	BaseComponent
	BaseDirtyMark
}

func NewDataComponent(entity Entity, componentName string) *DataComponent {
	return &DataComponent{
		BaseComponent: BaseComponent{
			entity: entity,
			name:   componentName,
		},
	}
}

type MapDataComponent struct {
	BaseComponent
	BaseMapDirtyMark
}

func NewMapDataComponent(entity Entity, componentName string) *MapDataComponent {
	return &MapDataComponent{
		BaseComponent: BaseComponent{
			entity: entity,
			name:   componentName,
		},
	}
}

// 获取玩家组件的缓存key
func GetPlayerComponentCacheKey(playerId int64, componentName string) string {
	// 使用{playerId}形式的hashtag,使同一个玩家的不同组件的数据都落在一个redis节点上
	// 落在一个redis节点上的好处:可以使用redis lua对玩家数据进行类似事务的原子操作
	// https://redis.io/topics/cluster-tutorial
	return fmt.Sprintf("p.%v.{%v}", strings.ToLower(componentName), playerId)
}

// 获取玩家组件子对象的缓存key
func GetPlayerComponentChildCacheKey(playerId int64, componentName string, childName string) string {
	return ""
}
