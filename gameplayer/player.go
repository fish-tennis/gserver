package gameplayer

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/db"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
)

var _ Entity = (*Player)(nil)

// 玩家对象
type Player struct {
	BaseEntity
	// 玩家唯一id
	id int64
	// 玩家名(unique)
	name string
	// 账号id
	accountId int64
	// 区服id
	regionId int32
	//accountName string
	//// 组件表
	//components []PlayerComponent
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
//func (this *player) GetComponent(componentId int) entity.Component {
//	for _,v := range this.components {
//		if v.GetId() == componentId {
//			return v
//		}
//	}
//	return nil
//}

// 获取组件
func (this *Player) GetComponent(componentName string) Component {
	index := GetComponentIndex(componentName)
	if index >= 0 {
		return this.GetComponentByIndex(index)
	}
	return nil
}

// 玩家数据保存数据库
func (this *Player) SaveDb(removeCacheAfterSaveDb bool) error {
	return SaveEntityToDb(db.GetPlayerDb(), this, removeCacheAfterSaveDb)
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
func (this *Player) Send(command PacketCommand, message proto.Message) bool {
	if this.connection != nil {
		return this.connection.Send(command, message)
	}
	return false
}

// 分发事件给组件
func (this *Player) FireEvent(event interface{}) {
	this.RangeComponent(func(component Component) bool {
		if eventReceiver, ok := component.(EventReceiver); ok {
			eventReceiver.OnEvent(event)
		}
		return true
	})
}

func (this *Player) GetGuild() *Guild {
	return this.GetComponent("Guild").(*Guild)
}

// 从加载的数据构造出玩家对象
func CreatePlayerFromData(playerData *pb.PlayerData) *Player {
	player := &Player{
		id:        playerData.Id,
		name:      playerData.Name,
		accountId: playerData.AccountId,
		regionId:  playerData.RegionId,
	}
	// 初始化玩家的各个模块
	player.AddComponent(NewBaseInfo(player, playerData.BaseInfo), nil)
	player.AddComponent(NewMoney(player), playerData.Money)
	bagCountItem := NewBagCountItem(player, playerData.BagCountItem)
	player.AddComponent(bagCountItem, nil)
	bagUniqueItem := NewBagUniqueItem(player)
	player.AddComponent(bagUniqueItem, playerData.BagUniqueItem)
	player.AddComponent(NewBag(player, bagCountItem, bagUniqueItem), nil)
	player.AddComponent(NewQuest(player), playerData.Quest)
	player.AddComponent(NewGuild(player), playerData.GuildData)
	return player
}

func CreateTempPlayer(playerId, accountId int64) *Player {
	playerData := &pb.PlayerData{}
	player := CreatePlayerFromData(playerData)
	player.id = playerId
	player.accountId = accountId
	return player
}
