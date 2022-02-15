package player

import (
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"reflect"
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
func (this *Player) GetComponent(componentName string) Component {
	index := GetComponentIndex(componentName)
	if index >= 0 {
		return this.components[index]
	}
	return nil
}

// 获取组件列表
func (this *Player) GetComponents() []Component {
	components := make([]Component, 0, len(this.components))
	for _,v := range this.components {
		components = append(components, v)
	}
	return components
}

// 保存所有修改过的组件数据到缓存
func (this *Player) SaveCache() error {
	for _,component := range this.components {
		if saveable,ok := component.(Saveable); ok {
			if dirtyMark,ok2 := component.(DirtyMark); ok2 {
				if !dirtyMark.IsDirty() {
					continue
				}
				cacheData := saveable.CacheData()
				if cacheData == nil {
					continue
				}
				cacheKeyName := GetComponentCacheKey(this.id, component.GetName())
				if reflect.ValueOf(cacheData).Kind() == reflect.Map {
					err := cache.Get().SetMap(cacheKeyName, cacheData)
					if err != nil {
						logger.Error("%v %v cache err:%v", this.id, component.GetNameLower(), err.Error())
					}
				} else {
					// 非map类型 必须是proto结构
					if protoMessage,ok := cacheData.(proto.Message); ok {
						err := cache.Get().SetProto(cacheKeyName, protoMessage, 0)
						if err != nil {
							logger.Error("%v %v cache err:%v", this.id, component.GetNameLower(), err.Error())
						}
					} else {
						logger.Error("%v %v cache err:unsupport type", this.id, component.GetNameLower())
					}
				}
				//cache.Get().Set(cacheKeyName, cacheData, 0)
				//cacheData,err := SaveWithProto(saveable, true)
				//if err != nil {
				//	continue
				//}
				//
				//if reflect.ValueOf(cacheData).Kind() == reflect.Map {
				//	_,cacheErr := cache.GetRedis().HSet(context.Background(), cacheKeyName, cacheData).Result()
				//	if cacheErr != nil {
				//		logger.Error("%v %v cache err:%v", this.id, component.GetNameLower(), cacheErr.Error())
				//		continue
				//	}
				//} else {
				//	_,cacheErr := cache.GetRedis().Set(context.Background(), cacheKeyName, cacheData, 0).Result()
				//	if cacheErr != nil {
				//		logger.Error("%v %v cache err:%v", this.id, component.GetNameLower(), cacheErr.Error())
				//		continue
				//	}
				//}
				dirtyMark.ResetDirty()
				logger.Debug("SaveCache %v %v", this.id, component.GetNameLower())
				continue
			}
			if dirtyMark,ok2 := component.(MapDirtyMark); ok2 {
				if !dirtyMark.IsDirty() {
					continue
				}
				cacheKeyName := GetComponentCacheKey(this.id, component.GetName())
				if !dirtyMark.HasCached() {
					// 必须把整体数据缓存一次,后面的修改才能局部更新
					cacheData := saveable.CacheData()
					if cacheData == nil {
						continue
					}
					err := cache.Get().SetMap(cacheKeyName, cacheData)
					if err != nil {
						logger.Error("%v %v cache err:%v", this.id, component.GetNameLower(), err.Error())
						continue
					}
					dirtyMark.SetCached()
				} else {
					if mapDataComponent,ok3 := component.(*MapDataComponent); ok3 {
						setMap := make(map[string]interface{})
						var delMap []string
						for dirtyKey,isAddOrUpdate := range mapDataComponent.dirtyMap {
							if isAddOrUpdate {
								if dirtyValue,exists := dirtyMark.GetMapValue(dirtyKey); exists {
									setMap[dirtyKey] = dirtyValue
									//_,err := cache.Get().SetMapField(cacheKeyName, dirtyKey, dirtyValue)
									//if err != nil {
									//	logger.Error("%v %v cache %v err:%v", this.id, component.GetNameLower(), dirtyKey, err.Error())
									//}
								}
							} else {
								// delete
								delMap = append(delMap, dirtyKey)
								//_,err := cache.GetRedis().HDel(context.Background(), cacheKeyName, dirtyKey).Result()
								//if err != nil {
								//	logger.Error("%v %v cache %v err:%v", this.id, component.GetNameLower(), dirtyKey, err.Error())
								//}
							}
						}
						if len(setMap) > 0 {
							err := cache.Get().SetMap(cacheKeyName, setMap)
							if err != nil {
								logger.Error("%v %v cache %v err:%v", this.id, component.GetNameLower(), setMap, err.Error())
							}
						}
						if len(delMap) > 0 {
							err := cache.Get().DelMapField(cacheKeyName, delMap...)
							if err != nil {
								logger.Error("%v %v cache %v err:%v", this.id, component.GetNameLower(), delMap, err.Error())
							}
						}
					}
				}
				//cacheData := saveable.Data(true)
				//if cacheData == nil {
				//	continue
				//}
				//cacheKeyName := GetComponentCacheKey(this.id, component.GetName())
				//err := cache.Get().SetMap(cacheKeyName, cacheData)
				//if err != nil {
				//	logger.Error("%v %v cache err:%v", this.id, component.GetNameLower(), err.Error())
				//	continue
				//}
				//_,cacheErr := cache.GetRedis().HSet(context.Background(), cacheKeyName, cacheData).Result()
				//if cacheErr != nil {
				//	logger.Error("%v %v cache err:%v", this.id, component.GetNameLower(), cacheErr.Error())
				//	continue
				//}
				dirtyMark.ResetDirty()
				logger.Debug("SaveCache %v %v", this.id, component.GetNameLower())
				continue
			}
		}
	}
	return nil
}

// 玩家数据保存数据库
func (this *Player) SaveDb(removeCacheAfterSaveDb bool) error {
	componentDatas := make(map[string]interface{})
	for _,component := range this.components {
		if saveable,ok := component.(Saveable); ok {
			// 如果组件数据没改变过,跳过保存
			if !saveable.IsChanged() {
				logger.Debug("%v ignore %v", this.id, component.GetName())
				continue
			}
			saveData,err := SaveWithProto(saveable)
			if err != nil {
				logger.Error("%v Save %v err:%v", this.id, component.GetName(), err.Error())
				continue
			}
			if saveData == nil {
				logger.Debug("%v ignore nil %v", this.id, component.GetName())
				continue
			}
			// 使用protobuf存mongodb时,mongodb默认会把字段名转成小写,因为protobuf没设置bson tag
			componentDatas[component.GetNameLower()] = saveData
			logger.Debug("SaveDb %v %v", this.id, component.GetName())
		}
		//if mapSaveable,ok := component.(MapSaveable); ok {
		//	if !mapSaveable.IsChanged() {
		//		continue
		//	}
		//	mapData,saveOption := mapSaveable.Save(true)
		//	if mapData == nil {
		//		continue
		//	}
		//	if saveOption == ProtoMarshalMap {
		//		for k,v := range mapData {
		//			if protoMessage,ok := v.(proto.Message); ok {
		//				bytes,err := proto.Marshal(protoMessage)
		//				if err != nil {
		//					logger.Error("Marshal %v %v %v err:%v", this.id, component.GetName(), k, err.Error())
		//				}
		//				mapData[k] = bytes
		//			}
		//		}
		//	}
		//	componentDatas[component.GetNameLower()] = mapData
		//	logger.Debug("SaveDb %v %v", this.id, component.GetName())
		//}
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
			cache.Get().Del(cacheKeyName)
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
		if eventReceiver,ok := component.(EventReceiver); ok {
			eventReceiver.OnEvent(event)
		}
	}
}

// 添加玩家组件
func (this *Player)addComponent(component PlayerComponent, bytes []byte) {
	if len(bytes) > 0 {
		if saveable,ok := component.(Saveable); ok {
			dbData,protoMarshal := saveable.DbData()
			if dbData != nil && protoMarshal {
				if protoMessage,ok2 := dbData.(proto.Message); ok2 {
					err := proto.Unmarshal(bytes, protoMessage)
					if err != nil {
						logger.Error("%v proto err:%v", component.GetName(), err)
					}
				}
			}
		}
	}
	// map[int]proto.Message格式的序列化
	this.components = append(this.components, component)
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
	player.addComponent(NewBaseInfo(player, playerData.BaseInfo), nil)
	player.addComponent(NewMoney(player), playerData.Money)
	//player.addComponent(NewBag(player), playerData.Bag)
	//player.addComponent(NewQuest(player), playerData.Quest)
	player.addComponent(NewBagCountItem(player, playerData.BagCountItem), nil)
	player.addComponent(NewBagUniqueItem(player, playerData.BagUniqueItem), nil)
	return player
}

func CreateTempPlayer(playerId,accountId int64) *Player {
	playerData := &pb.PlayerData{}
	player := CreatePlayerFromData(playerData)
	player.id = playerId
	player.accountId = accountId
	return player
}
