package game

import (
	"github.com/fish-tennis/gserver/entity"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
)

// 玩家对象
type Player struct {
	// 玩家唯一id
	id int64
	// 玩家名(unique)
	name string
	// 账号id
	accountId int64
	// 区服id
	regionId int32
	//accountName string
	// 组件表
	// TODO:用map是否更合适?
	components []PlayerComponent
	// 关联的连接
	connection Connection
}

// 玩家唯一id
func (this *Player) GetId() int64 {
	return this.id
}

// 玩家名(unique)
func (this *Player) GetName() string {
	return this.name
}

// 账号id
func (this *Player) GetAccountId() int64 {
	return this.accountId
}

// 区服id
func (this *Player) GetRegionId() int32 {
	return this.regionId
}

//// 获取组件
//func (this *Player) GetComponent(componentId int) entity.Component {
//	for _,v := range this.components {
//		if v.GetId() == componentId {
//			return v
//		}
//	}
//	return nil
//}

// 获取组件
func (this *Player) GetComponent(componentName string) entity.Component {
	for _,v := range this.components {
		if v.GetName() == componentName {
			return v
		}
	}
	return nil
}

// 获取组件列表
func (this *Player) GetComponents() []entity.Component {
	components := make([]entity.Component, 0, len(this.components))
	for _,v := range this.components {
		components = append(components, v)
	}
	return components
}

// 保存所有修改过的组件数据
func (this *Player) Save() error {
	for _,component := range this.components {
		if saveable,ok := component.(entity.Saveable); ok {
			err := saveable.Save()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// 设置关联的连接
func (this *Player) SetConnection(connection Connection) {
	// 取消之前的连接和该玩家的关联
	if this.connection != nil && this.connection != connection {
		this.connection.SetTag(nil)
	}
	this.connection = connection
}

func (this *Player) GetConnection() Connection {
	return this.connection
}

// 发包(protobuf)
// NOTE:调用Send(command,message)之后,不要再对message进行读写!
func (this *Player) Send(command Cmd, message proto.Message) bool {
	if this.connection != nil {
		return this.connection.Send(command, message)
	}
	return false
}

// 分发事件给组件
func (this *Player) FireEvent(event interface{}) {
	for _,component := range this.components {
		if eventReceiver,ok := component.(EventReceiver); ok {
			eventReceiver.OnEvent(event)
		}
	}
}

// 从加载的数据构造出玩家对象
func CreatePlayerFromData(playerData *pb.PlayerData) *Player {
	player := &Player{
		id: playerData.GetId(),
		name: playerData.GetName(),
		accountId: playerData.GetAccountId(),
		regionId: playerData.GetRegionId(),
	}
	// 初始化玩家的各个模块
	player.components = append(player.components, NewBaseInfo(player, playerData.BaseInfo))
	player.components = append(player.components, NewMoney(player, playerData.Money))
	return player
}
