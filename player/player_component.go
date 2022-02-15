package player

import (
	"fmt"
	"strings"

	. "github.com/fish-tennis/gserver/internal"
)

// 玩家组件接口
type PlayerComponent interface {
	Component
	// 关联的玩家对象
	GetPlayer() *Player
}

// 玩家组件
type BaseComponent struct {
	Player *Player
	// 组件名
	Name string
}

// 组件名
func (this *BaseComponent) GetName() string {
	return this.Name
}

func (this *BaseComponent) GetNameLower() string {
	return strings.ToLower(this.Name)
}

// entity.Component.GetEntity()的实现
func (this *BaseComponent) GetEntity() Entity {
	return this.Player
}

// 关联的玩家对象
func (this *BaseComponent) GetPlayer() *Player {
	return this.Player
}

// 关联的玩家id
func (this *BaseComponent) GetPlayerId() int64 {
	if this.Player == nil {
		return 0
	}
	return this.Player.GetId()
}

func (this *BaseComponent) GetCacheKey() string {
	return GetComponentCacheKey(this.GetPlayerId(), this.GetName())
}

// 有保存数据的玩家组件
type DataComponent struct {
	BaseComponent
	isChanged bool
	// 保存数据的修改标记
	isDirty bool
}

func NewDataComponent(player *Player, componentName string) *DataComponent {
	return &DataComponent{
		BaseComponent: BaseComponent{
			Player: player,
			Name:   componentName,
		},
	}
}

// 数据是否改变过
func (this *DataComponent) IsChanged() bool {
	return this.isChanged
}

func (this *DataComponent) IsDirty() bool {
	return this.isDirty
}

func (this *DataComponent) SetDirty() {
	this.isDirty = true
	this.isChanged = true
}

func (this *DataComponent) ResetDirty() {
	this.isDirty = false
}

// 获取玩家组件的缓存key
func GetComponentCacheKey(playerId int64, componentName string) string {
	return fmt.Sprintf("player.%v.{%v}", strings.ToLower(componentName), playerId)
}

// 有保存数据的玩家组件
type MapDataComponent struct {
	BaseComponent
	isChanged bool
	hasCached bool
	dirtyMap map[string]bool
}

func NewMapDataComponent(player *Player, componentName string) *MapDataComponent {
	return &MapDataComponent{
		BaseComponent: BaseComponent{
			Player: player,
			Name:   componentName,
		},
	}
}

func (this *MapDataComponent) IsChanged() bool {
	return this.isChanged
}

// 需要保存的数据是否修改了
func (this *MapDataComponent) IsDirty() bool {
	return len(this.dirtyMap) > 0
}

// 设置数据修改标记
func (this *MapDataComponent) SetDirty(k string, isAddOrUpdate bool) {
	if this.dirtyMap == nil {
		this.dirtyMap = make(map[string]bool)
	}
	this.dirtyMap[k] = isAddOrUpdate
	this.isChanged = true
}

// 重置标记
func (this *MapDataComponent) ResetDirty() {
	this.dirtyMap = make(map[string]bool)
}

// 是否把整体数据缓存了
func (this *MapDataComponent) HasCached() bool {
	return this.hasCached
}

func (this *MapDataComponent) SetCached() {
	this.hasCached = true
}

// TODO: MapInt32Component MapInt64Component MapStringComponent