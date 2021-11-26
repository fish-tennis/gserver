package game

import (
	"github.com/fish-tennis/gserver/entity"
	"github.com/fish-tennis/gserver/pb"
)

// 玩家对象
type Player struct {
	id int64
	name string
	accountId int64
	regionId int32
	//accountName string
	components []PlayerComponent
	playerData *pb.PlayerData
}

// 玩家组件接口
type PlayerComponent interface {
	entity.Component
	GetPlayer() *Player
	// 需要保存的数据
	DbData() interface{}
}

// 玩家组件
type BaseComponent struct {
	Player *Player
}

func (this *BaseComponent) GetId() int {
	return 0
}

func (this *BaseComponent) GetName() string {
	return ""
}

func (this *BaseComponent) GetEntity() entity.Entity {
	return this.Player
}

func (this *BaseComponent) GetPlayer() *Player {
	return this.Player
}

//// 需要保存的数据
//func (this *BaseComponent) DbData() interface{} {
//	return nil
//}
//
//func (this *BaseComponent) Save() error {
//	dbData := this.DbData()
//	if dbData == nil {
//		return nil
//	}
//	return GetServer().GetDb().SaveFieldInt64(this.Player.GetId(), this.GetName(), dbData)
//}

//func (this *BaseComponent) Load() error {
//	dbData := this.DbData()
//	if dbData == nil {
//		return nil
//	}
//	_,err := GetServer().GetDb().LoadFieldInt64(this.Player.GetId(), this.GetName(), dbData)
//	return err
//}

func (this *Player) GetId() int64 {
	return this.id
}

func (this *Player) GetName() string {
	return this.name
}

func (this *Player) GetAccountId() int64 {
	return this.accountId
}

func (this *Player) GetRegionId() int32 {
	return this.regionId
}

func (this *Player) GetComponent(componentId int) entity.Component {
	for _,v := range this.components {
		if v.GetId() == componentId {
			return v
		}
	}
	return nil
}

func (this *Player) GetComponents() []entity.Component {
	components := make([]entity.Component, 0, len(this.components))
	for _,v := range this.components {
		components = append(components, v)
	}
	return components
}

func (this *Player) SaveComponent(component PlayerComponent) error {
	//component.Save()
	dbData := component.DbData()
	if dbData == nil {
		return nil
	}
	return GetServer().GetDb().SaveFieldInt64(this.GetId(), component.GetName(), dbData)
}

func (this *Player) Save() error {
	for _,v := range this.components {
		err := this.SaveComponent(v)
		if err != nil {
			return err
		}
	}
	return nil
}

//func (this *Player) InitComponent() {
//	this.components = append(this.components, component.NewBaseInfo(this, nil))
//	//for _,v := range this.components {
//	//	err := v.Load()
//	//	if err != nil {
//	//		gnet.LogError("%s load err:%v", v.GetName(), err)
//	//		return
//	//	}
//	//}
//}

func CreatePlayerFromData(playerData *pb.PlayerData) *Player {
	player := &Player{
		id: playerData.GetId(),
		name: playerData.GetName(),
		accountId: playerData.GetAccountId(),
		regionId: playerData.GetRegionId(),
		playerData: playerData,
	}
	player.components = append(player.components, NewBaseInfo(player, playerData))
	return player
}
