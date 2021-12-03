package game

import (
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/entity"
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
	//// 组件id
	//id int
	// 组件名
	name string
}

//// 组件id
//func (this *BaseComponent) GetId() int {
//	return this.id
//}

// 组件名
func (this *BaseComponent) GetName() string {
	return this.name
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
	return this.Player.GetId()
}

// 有保存数据的玩家组件
type DataComponent struct {
	//entity.Saveable
	BaseComponent
	// 保存数据的修改标记
	isDirty bool
	// 保存数据接口
	dataFun func() interface{}
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

// 保存数据,保存成功后,重置dirty
func (this *DataComponent) Save() error {
	if this.isDirty && this.dataFun != nil {
		saveData := this.dataFun()
		err := GetServer().GetPlayerDb().SaveComponent(this.GetPlayerId(), this.GetName(), saveData)
		if err == nil {
			// 保存成功后,重置dirty
			this.ResetDirty()
			return nil
		}
		gnet.LogError("%v %v save err:%v", this.GetPlayerId(), this.GetName(), err)
		return err
	}
	return nil
}
