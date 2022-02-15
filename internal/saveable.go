package internal

import (
	"errors"
	"github.com/fish-tennis/gserver/logger"
	"google.golang.org/protobuf/proto"
	"reflect"
)

type SaveOption int

const (
	Plain SaveOption = iota
	ProtoMarshal
	PlainMap
	ProtoMarshalMap
)

// 保存数据接口
type Saveable interface {
	//// 保存数据
	//// saveData 需要保存的数据
	//Save(forCache bool) (saveData interface{}, saveOption SaveOption)
	////// 加载数据(反序列化)
	////Load(data interface{}, fromCache bool) error
	// 数据是否改变过
	IsChanged() bool

	// 需要保存到数据库的数据
	// 支持类型:
	// proto.Message
	// map key:int or string value:int or string or proto.Message
	DbData() (dbData interface{}, protoMarshal bool)

	// 需要缓存的数据
	// 支持类型:
	// proto.Message
	// map key:int or string value:int or string or proto.Message
	CacheData() interface{}
}

//type MapSaveable interface {
//	Save(forCache bool) (mapData map[string]interface{}, saveOption SaveOption)
//	// 加载数据(反序列化)
//	Load(data interface{}) error
//	// 数据是否改变过
//	IsChanged() bool
//}

type DirtyMark interface {
	// 需要保存的数据是否修改了
	IsDirty() bool
	// 设置数据修改标记
	SetDirty()
	// 重置标记
	ResetDirty()
}

type MapDirtyMark interface {
	// 需要保存的数据是否修改了
	IsDirty() bool
	// 设置数据修改标记
	SetDirty(k string, isAddOrUpdate bool)
	// 重置标记
	ResetDirty()

	// 是否把整体数据缓存了
	HasCached() bool
	SetCached()

	GetMapValue(key string) (value interface{}, exists bool)
}

// 保存数据,如果设置了对proto进行序列化,则进行序列化处理
func SaveWithProto(saveable Saveable) (interface{},error) {
	saveData,protoMarshal := saveable.DbData()
	if protoMarshal {
		// 对proto.Message进行序列化处理
		if protoMessage,ok := saveData.(proto.Message); ok {
			return proto.Marshal(protoMessage)
		}
		val := reflect.ValueOf(saveData)
		switch val.Kind() {
		case reflect.Interface,reflect.Ptr:
			// 保存proto格式
			if protoMessage,ok := saveData.(proto.Message); ok {
				return proto.Marshal(protoMessage)
			} else {
				logger.Error("unsupport type:%v", saveData)
				return nil, errors.New("unsupport type")
			}
		case reflect.Map:
			// 保存map格式
			typ := reflect.TypeOf(saveData)
			keyType := typ.Key()
			valType := typ.Elem()
			if valType.Kind() == reflect.Interface || valType.Kind() == reflect.Ptr {
				// map的value是proto格式
				switch keyType.Kind() {
				case reflect.Int,reflect.Int8,reflect.Int16,reflect.Int32,reflect.Int64:
					newMap := make(map[int64]interface{})
					it := val.MapRange()
					for it.Next() {
						newMap[it.Key().Int()] = it.Value().Interface()
					}
					return newMap,nil
				case reflect.Uint,reflect.Uint8,reflect.Uint16,reflect.Uint32,reflect.Uint64:
					newMap := make(map[uint64]interface{})
					it := val.MapRange()
					for it.Next() {
						newMap[it.Key().Uint()] = it.Value().Interface()
					}
					return newMap,nil
				case reflect.String:
					newMap := make(map[string]interface{})
					it := val.MapRange()
					for it.Next() {
						newMap[it.Key().String()] = it.Value().Interface()
					}
					return newMap,nil
				}
			} else {
				return saveData,nil
			}
		}
	}
	return saveData,nil
}

//// 加载数据,并对proto进行反序列化处理
//func LoadWithProto(fromData interface{}, toProtoMessage proto.Message) error {
//	if t,ok := fromData.([]byte); ok {
//		err := proto.Unmarshal(t, toProtoMessage)
//		if err != nil {
//			logger.Error("LoadWithProto err:%v", err.Error())
//			return err
//		}
//	}
//	return nil
//}

//func SaveCache(saveable Saveable, cacheKeyName string) error {
//	cacheData,err := SaveWithProto(saveable, true)
//	if err != nil {
//		return err
//	}
//	// map类型存为redis的hash表
//	if reflect.ValueOf(cacheData).Kind() == reflect.Map {
//		_,cacheErr := cache.GetRedis().HSet(context.Background(), cacheKeyName, cacheData).Result()
//		if cacheErr != nil {
//			logger.Error("%v cache err:%v", cacheKeyName, cacheErr.Error())
//			return cacheErr
//		}
//	} else {
//		_,cacheErr := cache.GetRedis().Set(context.Background(), cacheKeyName, cacheData, 0).Result()
//		if cacheErr != nil {
//			logger.Error("%v cache err:%v", cacheKeyName, cacheErr.Error())
//			return cacheErr
//		}
//	}
//	logger.Debug("SaveCache %v v:%v", cacheKeyName, cacheData)
//	return nil
//}
