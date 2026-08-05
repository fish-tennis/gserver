package game

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
)

const (
	// GlobalEntity在redis里的前缀
	GlobalEntityCachePrefix = db.GlobalDbName
	// GlobalEntity在mongo表里的key前缀
	GlobalEntityCollectionKeyPrefix = "GlobalEntity"
)

var (
	// GlobalEntity组件注册表
	_globalEntityComponentRegister = gentity.ComponentRegister[*GlobalEntity]{}
	// GlobalEntity消息回调接口注册
	_globalEntityPacketHandlerMgr = internal.NewPacketHandlerMgr()
)

// 演示全局类的非玩家实体
// 这里演示的GlobalEntity,每个game进程一个实例
type GlobalEntity struct {
	gentity.BaseRoutineEntity
	// 保存在global表中
	globalDb gentity.EntityDb
	// global表中的key
	key string
}

func NewGlobalEntity() *GlobalEntity {
	return &GlobalEntity{
		BaseRoutineEntity: *gentity.NewRoutineEntity(32),
		// NOTE: 需要在服务器初始化时调用mongoDb.RegisterEntityDb("global", "key")
		globalDb: db.GetGlobalDb(),
		key:      fmt.Sprintf("%v%v", GlobalEntityCollectionKeyPrefix, gentity.GetApplication().GetId()),
	}
}

func (this *GlobalEntity) LoadData(data interface{}) error {
	_, err := this.globalDb.FindEntityById(this.key, data)
	return err
}

func (this *GlobalEntity) SaveCache(kvCache gentity.KvCache) error {
	// redis中的key
	return this.BaseEntity.SaveCache(kvCache, GlobalEntityCachePrefix, this.key)
}

func (this *GlobalEntity) SaveDb(removeCacheAfterSaveDb bool) error {
	return gentity.SaveEntityChangedDataToDbByKey(this.globalDb, this, this.key,
		cache.Get(), removeCacheAfterSaveDb, GlobalEntityCachePrefix)
}

func (this *GlobalEntity) checkDataDirty() {
	// 对于非玩家实体,数据修改后是保存缓存还是直接保存数据库,需要根据实际业务需求来决定
	// 保存缓存不是必须的
	// 这里直接保存数据库了
	this.SaveDb(false)
	//this.SaveCache(cache.Get())
}

func (this *GlobalEntity) RunRoutine() bool {
	slog.Debug("GlobalEntity.RunRoutine", "key", this.key)
	ok := this.RunProcessRoutine(this, &gentity.RoutineEntityRoutineArgs{
		EndFunc: func(routineEntity gentity.RoutineEntity) {
			slog.Debug("GlobalEntity.RoutineEnd", "key", this.key)
		},
		ProcessMessageFunc: func(routineEntity gentity.RoutineEntity, message any) {
			if packet, ok := message.(*ProtoPacket); ok {
				this.processMessage(packet)
			} else {
				slog.Error(fmt.Sprintf("GlobalEntity ProcessMessage invalid type: %T", message))
			}
			this.checkDataDirty()
		},
		AfterTimerExecuteFunc: func(routineEntity gentity.RoutineEntity, t time.Time) {
			this.checkDataDirty()
		},
	})
	return ok
}

func (this *GlobalEntity) processMessage(message *ProtoPacket) {
	defer func() {
		if err := recover(); err != nil {
			slog.Error("GlobalEntity.processMessage recover", "error", err)
			LogStack()
			internal.SendAlert(err)
		}
	}()
	slog.Debug("GlobalEntity.processMessage", "message", proto.MessageName(message.Message()).Name())
	// 先找注册的消息回调接口
	if _globalEntityPacketHandlerMgr.Invoke(this, message, nil) {
		return
	}
	slog.Error("GlobalEntity.processMessage: unhandled message", "command", message.Command())
}

// 从数据库加载的数据构造出GlobalEntity对象
func CreateGlobalEntityFromDb() *GlobalEntity {
	globalEntity := NewGlobalEntity()
	globalEntityData := &pb.GlobalEntityData{}
	has, err := globalEntity.globalDb.FindEntityById(globalEntity.key, globalEntityData)
	globalEntity = createGlobalEntityFromData(globalEntity, globalEntityData)
	if err == nil && !has {
		// 数据库还没数据,则插入一条新数据
		newData := make(map[string]interface{})
		newData[db.GlobalDbKeyName] = globalEntity.key
		gentity.GetEntitySaveData(globalEntity, newData)
		if insertErr, _ := globalEntity.globalDb.InsertEntity(globalEntity.key, newData); insertErr != nil {
			slog.Error("GlobalEntity InsertEntity err", "err", insertErr, "key", globalEntity.key)
		}
	}
	return globalEntity
}

func createTempGlobalEntity() *GlobalEntity {
	globalEntity := NewGlobalEntity()
	globalEntityData := &pb.GlobalEntityData{}
	return createGlobalEntityFromData(globalEntity, globalEntityData)
}

func createGlobalEntityFromData(globalEntity *GlobalEntity, globalEntityData *pb.GlobalEntityData) *GlobalEntity {
	// 初始化各个模块
	_globalEntityComponentRegister.InitComponents(globalEntity, nil)
	if globalEntityData.Key != "" {
		err := gentity.LoadEntityData(globalEntity, globalEntityData)
		if err != nil {
			slog.Error("GlobalEntity LoadEntityDataErr", "key", globalEntityData.Key, "err", err)
		}
		globalEntity.RangeComponent(func(component gentity.Component) bool {
			if dataLoader, ok := component.(internal.DataLoader); ok {
				dataLoader.OnDataLoad()
				slog.Debug("GlobalEntity.OnDataLoad", "gid", globalEntity.GetId(), "component", component.GetName())
			}
			return true
		})
	}
	return globalEntity
}

// 注册GlobalEntity的结构体和消息回调
func InitGlobalEntityStructAndHandler() {
	tmpGlobalEntity := createTempGlobalEntity()
	gentity.ParseEntitySaveableStruct(tmpGlobalEntity)
	_globalEntityPacketHandlerMgr.AutoRegister(tmpGlobalEntity, internal.HandlerMethodNamePrefix)
}
