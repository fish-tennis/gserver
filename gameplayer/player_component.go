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

func (this *BasePlayerComponent) GetEntity() Entity {
	return this.player
}

func (this *BasePlayerComponent) SetEntity(entity Entity) {
	if v,ok := entity.(*Player); ok {
		this.player = v
	}
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

// 组件缓存key
func (this *BasePlayerComponent) GetCacheKey() string {
	return GetComponentCacheKey(this.GetPlayerId(), this.GetName())
}


// 保存数据作为一个整体的玩家组件
// 当保存数据的任何一个字段更新时,作为一个整体进行缓存更新
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


// 保存数据为map格式的玩家组件
// 当对map的某一项增删改时,只对那一项进行缓存更新
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

// 获取玩家组件的缓存key
func GetComponentCacheKey(playerId int64, componentName string) string {
	// 使用{playerId}形式的hashtag,使同一个玩家的不同组件的数据都落在一个redis节点上
	// 落在一个redis节点上的好处:可以使用redis lua对玩家数据进行类似事务的原子操作
	// https://redis.io/topics/cluster-tutorial
	return fmt.Sprintf("p.%v.{%v}", strings.ToLower(componentName), playerId)
}