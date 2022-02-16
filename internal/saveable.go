package internal

import (
	"errors"
	"github.com/fish-tennis/gserver/logger"
	"google.golang.org/protobuf/proto"
	"reflect"
)

// 保存数据接口
type Saveable interface {
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

// 保存数据作为一个整体,只要一个字段修改了,整个数据都需要缓存
type DirtyMark interface {
	// 需要保存的数据是否修改了
	IsDirty() bool
	// 设置数据修改标记
	SetDirty()
	// 重置标记
	ResetDirty()
}

// map格式的保存数据
// 第一次有数据修改时,会把整体数据缓存一次,之后只保存修改过的项
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
		//case reflect.Interface,reflect.Ptr:
		//	// 保存proto格式
		//	if protoMessage,ok := saveData.(proto.Message); ok {
		//		return proto.Marshal(protoMessage)
		//	} else {
		//		logger.Error("unsupport type:%v", saveData)
		//		return nil, errors.New("unsupport type")
		//	}
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
						if protoMessage,ok := it.Value().Interface().(proto.Message); ok {
							bytes,err := proto.Marshal(protoMessage)
							if err != nil {
								logger.Error("proto %v err:%v", it.Key().Int(), err.Error())
								return nil, err
							}
							newMap[it.Key().Int()] = bytes
						} else {
							newMap[it.Key().Int()] = it.Value().Interface()
						}
					}
					return newMap,nil
				case reflect.Uint,reflect.Uint8,reflect.Uint16,reflect.Uint32,reflect.Uint64:
					newMap := make(map[uint64]interface{})
					it := val.MapRange()
					for it.Next() {
						if protoMessage,ok := it.Value().Interface().(proto.Message); ok {
							bytes,err := proto.Marshal(protoMessage)
							if err != nil {
								logger.Error("proto %v err:%v", it.Key().Uint(), err.Error())
								return nil, err
							}
							newMap[it.Key().Uint()] = bytes
						} else {
							newMap[it.Key().Uint()] = it.Value().Interface()
						}
					}
					return newMap,nil
				case reflect.String:
					newMap := make(map[string]interface{})
					it := val.MapRange()
					for it.Next() {
						if protoMessage,ok := it.Value().Interface().(proto.Message); ok {
							bytes,err := proto.Marshal(protoMessage)
							if err != nil {
								logger.Error("proto %v err:%v", it.Key().String(), err.Error())
								return nil, err
							}
							newMap[it.Key().String()] = bytes
						} else {
							newMap[it.Key().String()] = it.Value().Interface()
						}
					}
					return newMap,nil
				default:
					logger.Error("unsupport key type:%v", keyType.Kind())
					return nil, errors.New("unsupport key type")
				}
			} else {
				return saveData,nil
			}
		}
	}
	return saveData,nil
}