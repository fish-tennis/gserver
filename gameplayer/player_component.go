package gameplayer

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
type BasePlayerComponent struct {
	player *Player
	// 组件名
	name string
}

// 组件名
func (this *BasePlayerComponent) GetName() string {
	return this.name
}

func (this *BasePlayerComponent) GetNameLower() string {
	return strings.ToLower(this.name)
}

// entity.Component.GetEntity()的实现
func (this *BasePlayerComponent) GetEntity() Entity {
	return this.player
}

// 关联的玩家对象
func (this *BasePlayerComponent) GetPlayer() *Player {
	return this.player
}

// 关联的玩家id
func (this *BasePlayerComponent) GetPlayerId() int64 {
	if this.player == nil {
		return 0
	}
	return this.player.GetId()
}

func (this *BasePlayerComponent) GetCacheKey() string {
	return GetComponentCacheKey(this.GetPlayerId(), this.GetName())
}

// 有保存数据的玩家组件
type PlayerDataComponent struct {
	BasePlayerComponent
	BaseDirtyMark
}

func NewPlayerDataComponent(player *Player, componentName string) *PlayerDataComponent {
	return &PlayerDataComponent{
		BasePlayerComponent: BasePlayerComponent{
			player: player,
			name:   componentName,
		},
	}
}

// 获取玩家组件的缓存key
func GetComponentCacheKey(playerId int64, componentName string) string {
	// 使用{playerId}形式的hashtag,使同一个玩家的不同组件的数据都落在一个redis节点上
	// https://redis.io/topics/cluster-tutorial
	return fmt.Sprintf("p.%v.{%v}", strings.ToLower(componentName), playerId)
}

// 有保存数据的玩家组件
type PlayerMapDataComponent struct {
	BasePlayerComponent
	BaseMapDirtyMark
}

func NewPlayerMapDataComponent(player *Player, componentName string) *PlayerMapDataComponent {
	return &PlayerMapDataComponent{
		BasePlayerComponent: BasePlayerComponent{
			player: player,
			name:   componentName,
		},
	}
}