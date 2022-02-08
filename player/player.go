package player

import (
	"context"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
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
	components []PlayerComponent
	// 关联的连接
	connection gnet.Connection
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
func (this *Player) GetComponent(componentName string) internal.Component {
	index := GetComponentIndex(componentName)
	if index >= 0 {
		return this.components[index]
	}
	return nil
}

// 获取组件列表
func (this *Player) GetComponents() []internal.Component {
	components := make([]internal.Component, 0, len(this.components))
	for _,v := range this.components {
		components = append(components, v)
	}
	return components
}

// 保存所有修改过的组件数据到缓存
func (this *Player) SaveCache() error {
	for _,component := range this.components {
		if saveable,ok := component.(internal.Saveable); ok {
			if !saveable.IsDirty() {
				continue
			}
			cacheData := saveable.Serialize(true)
			if cacheData == nil {
				continue
			}
			cacheKeyName := GetComponentCacheKey(this.id, component.GetName())
			_,cacheErr := cache.Get().Set(context.Background(), cacheKeyName, cacheData, 0).Result()
			if cacheErr != nil {
				logger.Error("%v %v cache err:%v", this.id, component.GetNameLower(), cacheErr.Error())
				continue
			}
			logger.Debug("SaveCache %v %v", this.id, component.GetNameLower())
		}
	}
	return nil
}

// 玩家数据保存数据库
func (this *Player) SaveDb(removeCacheAfterSaveDb bool) error {
	componentDatas := make(map[string]interface{})
	for _,component := range this.components {
		if saveable,ok := component.(internal.Saveable); ok {
			// 使用protobuf存mongodb时,mongodb默认会把字段名转成小写,因为protobuf没设置bson tag
			componentDatas[component.GetNameLower()] = saveable.Serialize(false)
			logger.Debug("SaveDb %v %v", this.id, component.GetName())
		}
	}
	saveDbErr := db.GetPlayerDb().SaveComponents(this.id, componentDatas)
	if saveDbErr != nil {
		logger.Error("SaveDb %v err:%v", this.id, saveDbErr)
	} else {
		logger.Debug("SaveDb %v", this.id)
	}
	if saveDbErr == nil && removeCacheAfterSaveDb {
		// 保存数据库成功后,删除缓存
		for k,_ := range componentDatas {
			cacheKeyName := GetComponentCacheKey(this.id, k)
			cache.Get().Del(context.Background(), cacheKeyName)
			logger.Debug("RemoveCache %v %v", this.id, cacheKeyName)
		}
	}
	return saveDbErr
}

// 设置关联的连接
func (this *Player) SetConnection(connection gnet.Connection) {
	// 取消之前的连接和该玩家的关联
	if this.connection != nil && this.connection != connection {
		this.connection.SetTag(nil)
	}
	this.connection = connection
}

func (this *Player) GetConnection() gnet.Connection {
	return this.connection
}

// 发包(protobuf)
// NOTE:调用Send(command,message)之后,不要再对message进行读写!
func (this *Player) Send(command gnet.PacketCommand, message proto.Message) bool {
	if this.connection != nil {
		return this.connection.Send(command, message)
	}
	return false
}

// 分发事件给组件
func (this *Player) FireEvent(event interface{}) {
	for _,component := range this.components {
		if eventReceiver,ok := component.(internal.EventReceiver); ok {
			eventReceiver.OnEvent(event)
		}
	}
}

// 添加玩家组件
func (this *Player)addComponent(component PlayerComponent) {
	this.components = append(this.components, component)
}

// 从加载的数据构造出玩家对象
func CreatePlayerFromData(playerData *pb.PlayerData) *Player {
	player := &Player{
		id: playerData.Id,
		name: playerData.Name,
		accountId: playerData.AccountId,
		regionId: playerData.RegionId,
	}
	// 初始化玩家的各个模块
	player.addComponent(NewBaseInfo(player, playerData.BaseInfo))
	player.addComponent(NewMoney(player, playerData.Money))
	player.addComponent(NewBag(player, playerData.Bag))
	return player
}

func CreateTempPlayer(playerId,accountId int64) *Player {
	playerData := &pb.PlayerData{}
	player := CreatePlayerFromData(playerData)
	player.id = playerId
	player.accountId = accountId
	return player
}
