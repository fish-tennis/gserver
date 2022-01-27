package game

import (
	"fmt"
	"github.com/fish-tennis/gserver/entity"
	"strings"
)

// 玩家组件接口
type PlayerComponent interface {
	entity.Component
	// 关联的玩家对象
	GetPlayer() *Player
}

// 玩家组件
type BaseComponent struct {
	Player *Player
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

// entity.Component.GetEntity()的实现
func (this *BaseComponent) GetEntity() entity.Entity {
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

// 有保存数据的玩家组件
type DataComponent struct {
	BaseComponent
	// 保存数据的修改标记
	isDirty bool
}

func (this *DataComponent) IsDirty() bool {
	return this.isDirty
}

func (this *DataComponent) SetDirty() {
	this.isDirty = true
}

func (this *DataComponent) ResetDirty() {
	this.isDirty = false
}

// 获取玩家组件的缓存key
func GetComponentCacheKey(playerId int64, componentName string) string {
	return fmt.Sprintf("player.%v.{%v}", strings.ToLower(componentName), playerId)
}