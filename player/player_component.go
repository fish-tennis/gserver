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
	BaseDirtyMark
}

func NewDataComponent(player *Player, componentName string) *DataComponent {
	return &DataComponent{
		BaseComponent: BaseComponent{
			Player: player,
			Name:   componentName,
		},
	}
}

// 获取玩家组件的缓存key
func GetComponentCacheKey(playerId int64, componentName string) string {
	return fmt.Sprintf("player.%v.{%v}", strings.ToLower(componentName), playerId)
}

// 有保存数据的玩家组件
type MapDataComponent struct {
	BaseComponent
	BaseMapDirtyMark
}

func NewMapDataComponent(player *Player, componentName string) *MapDataComponent {
	return &MapDataComponent{
		BaseComponent: BaseComponent{
			Player: player,
			Name:   componentName,
		},
	}
}

// TODO: MapInt32Component MapInt64Component MapStringComponent