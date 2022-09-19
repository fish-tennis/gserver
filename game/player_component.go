package game

import (
	"github.com/fish-tennis/gentity"
	"strings"
)

// 玩家组件接口
type PlayerComponent interface {
	gentity.Component
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

func (this *BasePlayerComponent) GetEntity() gentity.Entity {
	return this.player
}

func (this *BasePlayerComponent) SetEntity(entity gentity.Entity) {
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
	return gentity.GetPlayerComponentCacheKey(this.GetPlayerId(), this.GetName())
}


// 保存数据作为一个整体的玩家组件
// 当保存数据的任何一个字段更新时,作为一个整体进行缓存更新
type PlayerDataComponent struct {
	BasePlayerComponent
	gentity.BaseDirtyMark
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
	gentity.BaseMapDirtyMark
}

func NewPlayerMapDataComponent(player *Player, componentName string) *PlayerMapDataComponent {
	return &PlayerMapDataComponent{
		BasePlayerComponent: BasePlayerComponent{
			player: player,
			name:   componentName,
		},
	}
}