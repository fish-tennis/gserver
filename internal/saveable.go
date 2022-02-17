package internal

import (
	"errors"
	"fmt"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/util"
	"google.golang.org/protobuf/proto"
	"reflect"
	"strings"
)

// 保存数据接口
type Saveable interface {
	// 数据是否改变过
	IsChanged() bool
	//
	//KeyName() string

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

type SaveableChild interface {
	Saveable
	Key() string
}

// 多个保存模块的组合
type CompositeSaveable interface {
	SaveableChildren() []SaveableChild
}

//type ChildSaveable struct {
//	dbData interface{}
//	cacheData interface{}
//	protoMarshal bool
//	isChanged bool
//	key string
//}
//
//func (this *ChildSaveable) Key() string {
//	return this.key
//}
//
//func (this *ChildSaveable) IsChanged() bool {
//	return this.isChanged
//}
//
//func (this *ChildSaveable) DbData() (dbData interface{}, protoMarshal bool) {
//	return this.dbData,this.protoMarshal
//}
//
//func (this *ChildSaveable) CacheData() interface{} {
//	return this.cacheData
//}

// 保存数据作为一个整体,只要一个字段修改了,整个数据都需要缓存
type DirtyMark interface {
	// 需要保存的数据是否修改了
	IsDirty() bool
	// 设置数据修改标记
	SetDirty()
	// 重置标记
	ResetDirty()
}

type BaseDirtyMark struct {
	// 数据是否修改过
	isChanged bool
	isDirty bool
}

// 数据是否改变过
func (this *BaseDirtyMark) IsChanged() bool {
	return this.isChanged
}

func (this *BaseDirtyMark) IsDirty() bool {
	return this.isDirty
}

func (this *BaseDirtyMark) SetDirty() {
	this.isDirty = true
}

func (this *BaseDirtyMark) ResetDirty() {
	this.isDirty = false
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

type BaseMapDirtyMark struct {
	isChanged bool
	hasCached bool
	dirtyMap map[string]bool
}

func (this *BaseMapDirtyMark) IsChanged() bool {
	return this.isChanged
}

func (this *BaseMapDirtyMark) IsDirty() bool {
	return len(this.dirtyMap) > 0
}

func (this *BaseMapDirtyMark) SetDirty(k string, isAddOrUpdate bool) {
	if this.dirtyMap == nil {
		this.dirtyMap = make(map[string]bool)
	}
	this.dirtyMap[k] = isAddOrUpdate
	this.isChanged = true
}

func (this *BaseMapDirtyMark) ResetDirty() {
	this.dirtyMap = make(map[string]bool)
}

func (this *BaseMapDirtyMark) HasCached() bool {
	return this.hasCached
}

func (this *BaseMapDirtyMark) SetCached() {
	this.hasCached = true
}

func (this *BaseMapDirtyMark) GetDirtyMap() map[string]bool {
	return this.dirtyMap
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

func LoadSaveable(saveable Saveable, sourceData interface{}) error {
	if util.IsNil(sourceData) {
		return nil
	}
	dbData,protoMarshal := saveable.DbData()
	if !protoMarshal || util.IsNil(dbData) {
		return nil
	}
	sourceTyp := reflect.TypeOf(sourceData)
	switch sourceTyp.Kind() {
	case reflect.Slice:
		// []byte -> proto.Message
		if bytes,ok := sourceData.([]byte); ok {
			if len(bytes) > 0 {
				if protoMessage,ok2 := dbData.(proto.Message); ok2 {
					err := proto.Unmarshal(bytes, protoMessage)
					if err != nil {
						logger.Error("proto err:%v", err)
						return err
					}
				}
			}
			return nil
		}

	case reflect.Map:
		// map[intORstring]intORstring -> map[intORstring]intORstring
		// map[intORstring][]byte -> map[intORstring]proto.Message
		sourceVal := reflect.ValueOf(sourceData)
		sourceKeyType := sourceTyp.Key()
		sourceValType := sourceTyp.Elem()
		typ := reflect.TypeOf(dbData)
		if typ.Kind() == reflect.Map {
			val := reflect.ValueOf(dbData)
			keyType := typ.Key()
			valType := typ.Elem()
			sourceIt := sourceVal.MapRange()
			for sourceIt.Next() {
				k := convertValueToInterface(sourceKeyType, keyType, sourceIt.Key())
				v := convertValueToInterface(sourceValType, valType, sourceIt.Value())
				val.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(v))
			}
			return nil
		}
	}
	logger.Error("unsupport type:%v",sourceTyp.Kind())
	return nil
}

func LoadCompositeSaveable(compositeSaveable CompositeSaveable, sourceData interface{}) error {
	if util.IsNil(sourceData) {
		return nil
	}
	sourceTyp := reflect.TypeOf(sourceData)
	// 如果是proto,先转换成map
	if sourceTyp.Kind() == reflect.Ptr {
		protoMessage,ok := sourceData.(proto.Message)
		if !ok {
			logger.Error("unsupport type:%v",sourceTyp.Kind())
			return errors.New(fmt.Sprintf("unsupport type:%v", sourceTyp.Kind()))
		}
		sourceData = convertProtoToMap(protoMessage)
		sourceTyp = reflect.TypeOf(sourceData)
	}
	if sourceTyp.Kind() != reflect.Map {
		logger.Error("unsupport type:%v",sourceTyp.Kind())
		return errors.New(fmt.Sprintf("unsupport type:%v", sourceTyp.Kind()))
	}
	sourceVal := reflect.ValueOf(sourceData)
	saveables := compositeSaveable.SaveableChildren()
	for _,saveable := range saveables {
		v := sourceVal.MapIndex(reflect.ValueOf(saveable.Key()))
		if !v.IsValid() {
			logger.Debug("saveable not exists:%v", saveable.Key())
			continue
		}
		switch v.Kind() {
		case reflect.Slice:
			if v.Type().Elem().Kind() != reflect.Uint8 {
				logger.Error("unsupport slice type:%v",v.Type().Elem().Kind())
				return errors.New(fmt.Sprintf("unsupport slice type:%v",v.Type().Elem().Kind()))
			}
			LoadSaveable(saveable, v.Bytes())
		case reflect.Interface,reflect.Ptr:
			LoadSaveable(saveable, v.Interface())
		}
	}
	return nil
}

// reflect.Value -> interface{}
func convertValueToInterface(srcType,dstType reflect.Type, v reflect.Value) interface{} {
	switch srcType.Kind() {
	case reflect.Int,reflect.Int8,reflect.Int16,reflect.Int32,reflect.Int64:
		return convertInterfaceToRealType(dstType, v.Int())
	case reflect.Uint,reflect.Uint8,reflect.Uint16,reflect.Uint32,reflect.Uint64:
		return convertInterfaceToRealType(dstType, v.Uint())
	case reflect.String:
		return convertInterfaceToRealType(dstType, v.String())
	case reflect.Interface,reflect.Ptr:
		return convertInterfaceToRealType(dstType, v.Interface())
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return convertInterfaceToRealType(dstType, v.Bytes())
		}
	}
	logger.Error("unsupport type:%v",srcType.Kind())
	return nil
}

// interface{} -> int or string or proto.Message
func convertInterfaceToRealType(typ reflect.Type, v interface{}) interface{} {
	switch typ.Kind() {
	case reflect.Int:
		return int(v.(int64))
	case reflect.Int8:
		return int8(v.(int64))
	case reflect.Int16:
		return int16(v.(int64))
	case reflect.Int32:
		return int32(v.(int64))
	case reflect.Int64:
		return v.(int64)
	case reflect.Uint:
		return uint(v.(uint64))
	case reflect.Uint8:
		return uint8(v.(uint64))
	case reflect.Uint16:
		return uint16(v.(uint64))
	case reflect.Uint32:
		return uint32(v.(uint64))
	case reflect.Uint64:
		return v.(uint64)
	case reflect.String:
		return v
	case reflect.Interface,reflect.Ptr,reflect.Slice:
		if bytes,ok := v.([]byte); ok {
			newProto := reflect.New(typ.Elem())
			if protoMessage,ok2 := newProto.Interface().(proto.Message); ok2 {
				protoErr := proto.Unmarshal(bytes, protoMessage)
				if protoErr != nil {
					return protoErr
				}
				return protoMessage
			}
		}
	}
	logger.Error("unsupport type:%v",typ.Kind())
	return nil
}

// proto.Message -> map[string]interface{}
func convertProtoToMap(protoMessage proto.Message) map[string]interface{} {
	stringMap := make(map[string]interface{})
	typ := reflect.TypeOf(protoMessage).Elem()
	val := reflect.ValueOf(protoMessage).Elem()
	for i := 0; i < typ.NumField(); i++ {
		sf := typ.Field(i)
		if len(sf.Tag) == 0 {
			continue
		}
		var v interface{}
		fieldVal := val.Field(i)
		switch fieldVal.Kind() {
		case reflect.Slice:
			if fieldVal.Type().Elem().Kind() == reflect.Uint8 {
				v = fieldVal.Bytes()
			}
		case reflect.Interface,reflect.Ptr,reflect.Map:
			v = fieldVal.Interface()
		}
		if v == nil {
			continue
		}
		stringMap[strings.ToLower(sf.Name)] = v
	}
	//protoMessage.ProtoReflect().Range(func(descriptor protoreflect.FieldDescriptor, value protoreflect.Value) bool {
	//	stringMap[string(descriptor.Name())] = value.Interface()
	//	return true
	//})
	return stringMap
}