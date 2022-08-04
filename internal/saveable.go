package internal

import (
	"errors"
	"fmt"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/util"
	"github.com/go-redis/redis/v8"
	"google.golang.org/protobuf/proto"
	"reflect"
	"strings"
)

// 保存数据接口
type Saveable interface {
	// 数据是否改变过
	IsChanged() bool

	ResetChanged()

	// 需要保存到数据库的数据
	// 支持类型:
	// proto.Message
	// map[intORstring]intORstring
	// map[intORstring]proto.Message
	// SliceInt32
	DbData() (dbData interface{}, protoMarshal bool)

	// 需要缓存的数据
	// 支持类型:
	// proto.Message
	// map key:int or string value:int or string or proto.Message
	CacheData() interface{}

	GetCacheKey() string
}

// 保存接口子模块
type ChildSaveable interface {
	Saveable
	Key() string
}

// 多个保存子模块的组合
type CompositeSaveable interface {
	SaveableChildren() []ChildSaveable
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

type SaveableDirtyMark interface {
	Saveable
	DirtyMark
}

type BaseDirtyMark struct {
	// 数据是否修改过
	isChanged bool
	isDirty   bool
}

// 数据是否改变过
func (this *BaseDirtyMark) IsChanged() bool {
	return this.isChanged
}

func (this *BaseDirtyMark) ResetChanged() {
	this.isChanged = false
}

func (this *BaseDirtyMark) IsDirty() bool {
	return this.isDirty
}

func (this *BaseDirtyMark) SetDirty() {
	this.isDirty = true
	this.isChanged = true
}

func (this *BaseDirtyMark) ResetDirty() {
	this.isDirty = false
}

// map格式的保存数据
// 第一次有数据修改时,会把整体数据缓存一次,之后只保存修改过的项(增量更新)
type MapDirtyMark interface {
	// 需要保存的数据是否修改了
	IsDirty() bool
	// 设置数据修改标记
	SetDirty(k interface{}, isAddOrUpdate bool)
	// 重置标记
	ResetDirty()

	// 是否把整体数据缓存了
	HasCached() bool
	SetCached()

	GetDirtyMap() map[string]bool
	GetMapValue(key string) (value interface{}, exists bool)
}

type SaveableMapDirtyMark interface {
	Saveable
	MapDirtyMark
}

type BaseMapDirtyMark struct {
	isChanged bool
	hasCached bool
	dirtyMap  map[string]bool
}

func (this *BaseMapDirtyMark) IsChanged() bool {
	return this.isChanged
}

func (this *BaseMapDirtyMark) ResetChanged() {
	this.isChanged = false
}

func (this *BaseMapDirtyMark) IsDirty() bool {
	return len(this.dirtyMap) > 0
}

func (this *BaseMapDirtyMark) SetDirty(k interface{}, isAddOrUpdate bool) {
	if this.dirtyMap == nil {
		this.dirtyMap = make(map[string]bool)
	}
	this.dirtyMap[util.Itoa(k)] = isAddOrUpdate
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

// 获取对象的保存数据
func GetSaveData(obj interface{}, parentName string) (interface{}, error) {
	structCache := GetSaveableStruct(reflect.TypeOf(obj))
	if structCache == nil {
		logger.Debug("not saveable %v", parentName)
		return nil, nil
	}
	reflectVal := reflect.ValueOf(obj).Elem()
	if !structCache.IsCompositeSaveable {
		fieldCache := structCache.Fields[0]
		val := reflectVal.Field(fieldCache.FieldIndex)
		if val.IsNil() {
			return nil, nil
		}
		fieldInterface := val.Interface()
		// 明文保存数据
		if fieldCache.IsPlain {
			return fieldInterface, nil
		}
		// 非明文保存的数据,一般用于对proto进行序列化
		switch val.Kind() {
		case reflect.Map:
			// 保存数据是一个map
			typ := reflect.TypeOf(fieldInterface)
			keyType := typ.Key()
			valType := typ.Elem()
			if valType.Kind() == reflect.Interface || valType.Kind() == reflect.Ptr {
				switch keyType.Kind() {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					// map[int]proto.Message -> map[int64][]byte
					// map[int]interface{} -> map[int64]interface{}
					newMap := make(map[int64]interface{})
					it := val.MapRange()
					for it.Next() {
						// map的value是proto格式,进行序列化
						if protoMessage, ok := it.Value().Interface().(proto.Message); ok {
							bytes, err := proto.Marshal(protoMessage)
							if err != nil {
								logger.Error("%v.%v proto %v err:%v", parentName, fieldCache.Name, it.Key().Int(), err.Error())
								return nil, err
							}
							newMap[it.Key().Int()] = bytes
						} else {
							newMap[it.Key().Int()] = it.Value().Interface()
						}
					}
					return newMap, nil
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					// map[uint]proto.Message -> map[uint64][]byte
					// map[uint]interface{} -> map[uint64]interface{}
					newMap := make(map[uint64]interface{})
					it := val.MapRange()
					for it.Next() {
						// map的value是proto格式,进行序列化
						if protoMessage, ok := it.Value().Interface().(proto.Message); ok {
							bytes, err := proto.Marshal(protoMessage)
							if err != nil {
								logger.Error("%v.%v proto %v err:%v", parentName, fieldCache.Name, it.Key().Uint(), err.Error())
								return nil, err
							}
							newMap[it.Key().Uint()] = bytes
						} else {
							newMap[it.Key().Uint()] = it.Value().Interface()
						}
					}
					return newMap, nil
				case reflect.String:
					// map[string]proto.Message -> map[string][]byte
					// map[string]interface{} -> map[string]interface{}
					newMap := make(map[string]interface{}, val.Len())
					it := val.MapRange()
					for it.Next() {
						// map的value是proto格式,进行序列化
						if protoMessage, ok := it.Value().Interface().(proto.Message); ok {
							bytes, err := proto.Marshal(protoMessage)
							if err != nil {
								logger.Error("%v.%v proto %v err:%v", parentName, fieldCache.Name, it.Key().String(), err.Error())
								return nil, err
							}
							newMap[it.Key().String()] = bytes
						} else {
							newMap[it.Key().String()] = it.Value().Interface()
						}
					}
					return newMap, nil
				default:
					logger.Error("%v.%v unsupport key type:%v", parentName, fieldCache.Name, keyType.Kind())
					return nil, errors.New("unsupport key type")
				}
			} else {
				// map的value是基础类型,无需序列化,直接返回
				return fieldInterface, nil
			}

		case reflect.Slice:
			typ := reflect.TypeOf(fieldInterface)
			valType := typ.Elem()
			if valType.Kind() == reflect.Interface || valType.Kind() == reflect.Ptr {
				newSlice := make([]interface{}, 0, val.Len())
				for i := 0; i < val.Len(); i++ {
					sliceElem := val.Index(i)
					if protoMessage, ok := sliceElem.Interface().(proto.Message); ok {
						bytes, err := proto.Marshal(protoMessage)
						if err != nil {
							logger.Error("%v.%v proto %v err:%v", parentName, fieldCache.Name, i, err.Error())
							return nil, err
						}
						newSlice = append(newSlice, bytes)
					} else {
						newSlice = append(newSlice, sliceElem.Interface())
					}
				}
				// proto
				return newSlice, nil
			} else {
				// slice的value是基础类型,无需序列化,直接返回
				return fieldInterface, nil
			}

		case reflect.Ptr:
			// 模块的保存数据是一个proto.Message
			// proto.Message -> []byte
			if protoMessage, ok := fieldInterface.(proto.Message); ok {
				return proto.Marshal(protoMessage)
			}
			if sliceInt32, ok := fieldInterface.(*SliceInt32); ok {
				return sliceInt32.Data(), nil
			}

		default:
			return nil, errors.New("unsupport key type")
		}
	} else {
		// 多个子模块的组合
		compositeSaveData := make(map[string]interface{})
		for _, fieldCache := range structCache.Fields {
			childName := parentName + "." + fieldCache.Name
			val := reflectVal.Field(fieldCache.FieldIndex)
			if val.IsNil() {
				compositeSaveData[fieldCache.Name] = nil
				continue
			}
			fieldInterface := val.Interface()
			childSaveData, err := GetSaveData(fieldInterface, childName)
			if err != nil {
				return nil, err
			}
			compositeSaveData[fieldCache.Name] = childSaveData
		}
		return compositeSaveData, nil
	}
	return nil, errors.New("unsupport type")
}

// 组件的保存数据
func GetComponentSaveData(component Component) (interface{}, error) {
	return GetSaveData(component, component.GetNameLower())
}

// 加载数据
// 反序列化
func LoadData(obj interface{}, sourceData interface{}) error {
	if util.IsNil(sourceData) {
		return nil
	}
	structCache := GetSaveableStruct(reflect.TypeOf(obj))
	if structCache == nil {
		return errors.New("not Saveable object")
	}
	reflectVal := reflect.ValueOf(obj).Elem()
	if !structCache.IsCompositeSaveable {
		fieldCache := structCache.Fields[0]
		val := reflectVal.Field(fieldCache.FieldIndex)
		if val.IsNil() {
			if !val.CanSet() {
				logger.Error("%v CanSet false", fieldCache.Name)
				return nil
			}
			newElem := reflect.New(fieldCache.StructField.Type)
			val.Set(newElem)
		}
		sourceTyp := reflect.TypeOf(sourceData)
		switch sourceTyp.Kind() {
		case reflect.Slice:
			if !fieldCache.IsPlain {
				// proto反序列化
				// []byte -> proto.Message
				if sourceTyp.Elem().Kind() == reflect.Uint8 {
					if bytes, ok := sourceData.([]byte); ok {
						if len(bytes) == 0 {
							return nil
						}
						// []byte -> proto.Message
						if protoMessage, ok2 := val.Interface().(proto.Message); ok2 {
							err := proto.Unmarshal(bytes, protoMessage)
							if err != nil {
								logger.Error("%v proto.Unmarshal err:%v", fieldCache.Name, err.Error())
								return err
							}
							return nil
						}
					}
				}
			}
			// 基础类型的slice
			if fieldCache.StructField.Type.Kind() == reflect.Slice {
				// 数组类型一致,就直接赋值
				if sourceTyp.Elem().Kind() == fieldCache.StructField.Type.Elem().Kind() {
					if val.CanSet() {
						val.Set(reflect.ValueOf(sourceData))
						return nil
					}
				}
				// 类型不一致,暂时返回错误
				return errors.New("slice element type error")
			}

		case reflect.Map:
			// map[intORstring]intORstring -> map[intORstring]intORstring
			// map[intORstring][]byte -> map[intORstring]proto.Message
			sourceVal := reflect.ValueOf(sourceData)
			sourceKeyType := sourceTyp.Key()
			sourceValType := sourceTyp.Elem()
			if fieldCache.StructField.Type.Kind() == reflect.Map {
				//fieldVal := reflect.ValueOf(val)
				keyType := fieldCache.StructField.Type.Key()
				valType := fieldCache.StructField.Type.Elem()
				sourceIt := sourceVal.MapRange()
				for sourceIt.Next() {
					k := convertValueToInterface(sourceKeyType, keyType, sourceIt.Key())
					v := convertValueToInterface(sourceValType, valType, sourceIt.Value())
					val.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(v))
				}
				return nil
			}

		case reflect.Interface, reflect.Ptr:
			sourceVal := reflect.ValueOf(sourceData)
			//if fieldCache.StructField.Type == sourceTyp {
			//	if val.CanSet() {
			//		val.Set(sourceVal)
			//		return nil
			//	}
			//}
			if sourceProtoMessage, ok := sourceVal.Interface().(proto.Message); ok {
				if protoMessage, ok2 := val.Interface().(proto.Message); ok2 {
					if sourceProtoMessage.ProtoReflect().Descriptor() == protoMessage.ProtoReflect().Descriptor() {
						proto.Merge(protoMessage, sourceProtoMessage)
						return nil
					}
				}
			}
			// TODO:扩展一个序列化接口
			return errors.New(fmt.Sprintf("unsupport type %v", fieldCache.Name))

		default:
			return errors.New(fmt.Sprintf("unsupport type %v sourceTyp.Kind():%v", fieldCache.Name, sourceTyp.Kind()))

		}
	} else {
		sourceTyp := reflect.TypeOf(sourceData)
		// 如果是proto,先转换成map
		if sourceTyp.Kind() == reflect.Ptr {
			protoMessage, ok := sourceData.(proto.Message)
			if !ok {
				logger.Error("unsupport type:%v", sourceTyp.Kind())
				return errors.New(fmt.Sprintf("unsupport type:%v", sourceTyp.Kind()))
			}
			// mongodb中读出来是proto.Message格式,转换成map[string]interface{}
			sourceData = convertProtoToMap(protoMessage)
			sourceTyp = reflect.TypeOf(sourceData)
		}
		if sourceTyp.Kind() != reflect.Map {
			logger.Error("unsupport type:%v", sourceTyp.Kind())
			return errors.New("sourceData type error")
		}
		sourceVal := reflect.ValueOf(sourceData)
		for _, fieldCache := range structCache.Fields {
			sourceFieldVal := sourceVal.MapIndex(reflect.ValueOf(fieldCache.Name))
			if !sourceFieldVal.IsValid() {
				logger.Debug("saveable not exists:%v", fieldCache.Name)
				continue
			}
			val := reflectVal.Field(fieldCache.FieldIndex)
			if val.IsNil() {
				if !val.CanSet() {
					logger.Error("child cant new field:%v", fieldCache.Name)
					continue
				}
				newElem := reflect.New(fieldCache.StructField.Type)
				val.Set(newElem)
			}
			fieldInterface := val.Interface()
			childLoadErr := LoadData(fieldInterface, sourceFieldVal.Interface())
			if childLoadErr != nil {
				logger.Error("child load error field:%v", fieldCache.Name)
				return childLoadErr
			}
		}
	}
	return nil
}

// reflect.Value -> interface{}
func convertValueToInterface(srcType, dstType reflect.Type, v reflect.Value) interface{} {
	switch srcType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return convertInterfaceToRealType(dstType, v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return convertInterfaceToRealType(dstType, v.Uint())
	case reflect.Float32, reflect.Float64:
		return convertInterfaceToRealType(dstType, v.Float())
	case reflect.String:
		return convertInterfaceToRealType(dstType, v.String())
	case reflect.Interface, reflect.Ptr:
		return convertInterfaceToRealType(dstType, v.Interface())
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return convertInterfaceToRealType(dstType, v.Bytes())
		} else {
			return convertInterfaceToRealType(dstType, v.Interface())
		}
	}
	logger.Error("unsupport type:%v", srcType.Kind())
	return nil
}

// reflect.Value -> int
func convertValueToInt(srcType reflect.Type, v reflect.Value) int64 {
	switch srcType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(v.Uint())
	case reflect.Float32, reflect.Float64:
		// NOTE:有精度问题
		return int64(v.Float())
	}
	logger.Error("unsupport type:%v", srcType.Kind())
	return 0
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
	case reflect.Float32:
		return v.(float32)
	case reflect.Float64:
		return v.(float64)
	case reflect.String:
		return v
	case reflect.Ptr, reflect.Slice:
		if bytes, ok := v.([]byte); ok {
			newProto := reflect.New(typ.Elem())
			if protoMessage, ok2 := newProto.Interface().(proto.Message); ok2 {
				protoErr := proto.Unmarshal(bytes, protoMessage)
				if protoErr != nil {
					return protoErr
				}
				return protoMessage
			}
		}
	}
	logger.Error("unsupport type:%v", typ.Kind())
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
			} else {
				v = fieldVal.Interface()
			}
		case reflect.Interface, reflect.Ptr, reflect.Map:
			v = fieldVal.Interface()
		}
		if v == nil {
			logger.Debug("%v %v nil", sf.Name, fieldVal.Kind())
			continue
		}
		// 兼容mongodb,字段名小写
		stringMap[strings.ToLower(sf.Name)] = v
	}
	return stringMap
}

func SaveComponentChangedDataToCache(component Component) {
	structCache := GetSaveableStruct(reflect.TypeOf(component))
	if structCache == nil {
		return
	}
	if !structCache.IsCompositeSaveable {
		cacheKey := GetPlayerComponentCacheKey(component.GetEntity().GetId(), component.GetName())
		SaveChangedDataToCache(component, cacheKey)
	} else {
		reflectVal := reflect.ValueOf(component).Elem()
		for _,fieldCache := range structCache.Fields {
			val := reflectVal.Field(fieldCache.FieldIndex)
			if val.IsNil() {

			}
			fieldInterface := val.Interface()
			cacheKey := GetPlayerComponentChildCacheKey(component.GetEntity().GetId(), component.GetName(), fieldCache.Name)
			SaveChangedDataToCache(fieldInterface, cacheKey)
		}
	}
}

// 把修改数据保存到缓存
func SaveChangedDataToCache(saveable interface{}, cacheKeyName string) {
	structCache := GetSaveableStruct(reflect.TypeOf(saveable))
	if structCache == nil {
		return
	}
	if structCache.IsCompositeSaveable {
		return
	}
	fieldCache := structCache.Fields[0]
	// 缓存数据作为一个整体的
	if dirtyMark, ok := saveable.(DirtyMark); ok {
		if !dirtyMark.IsDirty() {
			return
		}
		reflectVal := reflect.ValueOf(saveable).Elem()
		val := reflectVal.Field(fieldCache.FieldIndex)
		if val.IsNil() {
			err := cache.Get().Del(cacheKeyName)
			if cache.IsRedisError(err) {
				logger.Error("%v cache err:%v", cacheKeyName, err.Error())
				return
			}
		} else {
			switch val.Kind() {
			case reflect.Ptr,reflect.Interface:
				cacheData := val.Interface()
				switch realData := cacheData.(type) {
				case proto.Message:
					// proto.Message -> []byte
					err := cache.Get().SetProto(cacheKeyName, realData, 0)
					if cache.IsRedisError(err) {
						logger.Error("%v cache err:%v", cacheKeyName, err.Error())
						return
					}

				case []int32:
					// TODO: []int32 -> string
				//case *SliceInt32:
				//	// SliceInt32 -> string
				//	err := cache.Get().Set(cacheKeyName, realData.ToString(), 0)
				//	if cache.IsRedisError(err) {
				//		logger.Error("%v cache err:%v", cacheKeyName, err.Error())
				//		return
				//	}

				default:
					logger.Error("%v cache err:unsupport type:%v", cacheKeyName, reflect.TypeOf(realData))
					return
				}

			case reflect.Map:
				// map格式作为一个整体缓存时,需要先删除之前的数据
				err := cache.Get().Del(cacheKeyName)
				if cache.IsRedisError(err) {
					logger.Error("%v cache err:%v", cacheKeyName, err.Error())
					return
				}
				cacheData := val.Interface()
				err = cache.Get().SetMap(cacheKeyName, cacheData)
				if cache.IsRedisError(err) {
					logger.Error("%v cache err:%v", cacheKeyName, err.Error())
					return
				}

			default:
				logger.Error("%v cache err:unsupport kind:%v", cacheKeyName, val.Kind())
			}
		}
		dirtyMark.ResetDirty()
		logger.Debug("SaveCache %v", cacheKeyName)
		return
	}
	// map格式的
	if dirtyMark, ok := saveable.(MapDirtyMark); ok {
		if !dirtyMark.IsDirty() {
			return
		}
		reflectVal := reflect.ValueOf(saveable).Elem()
		val := reflectVal.Field(fieldCache.FieldIndex)
		if val.IsNil() {
			err := cache.Get().Del(cacheKeyName)
			if cache.IsRedisError(err) {
				logger.Error("%v cache err:%v", cacheKeyName, err.Error())
				return
			}
		} else {
			if val.Kind() != reflect.Map {
				logger.Error("%v unsupport kind:%v", cacheKeyName, val.Kind())
				return
			}
			cacheData := val.Interface()
			if !dirtyMark.HasCached() {
				// 必须把整体数据缓存一次,后面的修改才能增量更新
				if cacheData == nil {
					return
				}
				err := cache.Get().SetMap(cacheKeyName, cacheData)
				if cache.IsRedisError(err) {
					logger.Error("%v cache err:%v", cacheKeyName, err.Error())
					return
				}
				dirtyMark.SetCached()
			} else {
				setMap := make(map[string]interface{})
				var delMap []string
				for dirtyKey, isAddOrUpdate := range dirtyMark.GetDirtyMap() {
					if isAddOrUpdate {
						if dirtyValue, exists := dirtyMark.GetMapValue(dirtyKey); exists {
							setMap[dirtyKey] = dirtyValue
						}
					} else {
						// delete
						delMap = append(delMap, dirtyKey)
					}
				}
				if len(setMap) > 0 {
					// 批量更新
					err := cache.Get().SetMap(cacheKeyName, setMap)
					if cache.IsRedisError(err) {
						logger.Error("%v cache %v err:%v", cacheKeyName, setMap, err.Error())
						return
					}
				}
				if len(delMap) > 0 {
					// 批量删除
					err := cache.Get().DelMapField(cacheKeyName, delMap...)
					if cache.IsRedisError(err) {
						logger.Error("%v cache %v err:%v", cacheKeyName, delMap, err.Error())
						return
					}
				}
			}
		}
		dirtyMark.ResetDirty()
		logger.Debug("SaveCache %v", cacheKeyName)
		return
	}
}

// 从缓存中恢复数据
func LoadFromCache(obj interface{}, cacheKey string) (bool, error) {
	structCache := GetSaveableStruct(reflect.TypeOf(obj))
	if structCache == nil {
		return false, nil
	}
	cacheType, err := cache.Get().Type(cacheKey)
	if err == redis.Nil || cacheType == "" || cacheType == "none" {
		return false, nil
	}
	reflectVal := reflect.ValueOf(obj).Elem()
	if !structCache.IsCompositeSaveable {
		fieldCache := structCache.Fields[0]
		val := reflectVal.Field(fieldCache.FieldIndex)
		if cacheType == "string" {
			if fieldCache.StructField.Type.Kind() == reflect.Ptr || fieldCache.StructField.Type.Kind() == reflect.Interface {
				if val.IsNil() {
					if !val.CanSet() {
						logger.Error("%v CanSet false", fieldCache.Name)
						return true, errors.New(fmt.Sprintf("%v CanSet false", fieldCache.Name))
					}
					newElem := reflect.New(fieldCache.StructField.Type)
					val.Set(newElem)
					logger.Debug("cacheKey:%v new %v", cacheKey, fieldCache.Name)
				}
				if !val.CanInterface() {
					return true, errors.New(fmt.Sprintf("%v CanInterface false", fieldCache.Name))
				}
				if protoMessage, ok := val.Interface().(proto.Message); ok {
					// []byte -> proto.Message
					err = cache.Get().GetProto(cacheKey, protoMessage)
					if cache.IsRedisError(err) {
						logger.Error("GetProto %v %v err:%v", cacheKey, cacheType, err)
						return true, err
					}
					return true, nil
				}
			} else if fieldCache.StructField.Type.Kind() == reflect.Slice {
				// TODO:slice int
			}
			return true, errors.New(fmt.Sprintf("unsupport kind:%v cacheKey:%v cacheType:%v", fieldCache.StructField.Type.Kind(), cacheKey, cacheType))
		} else if cacheType == "hash" {
			if val.IsNil() {
				if !val.CanSet() {
					logger.Error("%v CanSet false", fieldCache.Name)
					return true, errors.New(fmt.Sprintf("%v CanSet false", fieldCache.Name))
				}
				newElem := reflect.New(fieldCache.StructField.Type)
				val.Set(newElem)
				logger.Debug("cacheKey:%v new %v", cacheKey, fieldCache.Name)
			}
			if !val.CanInterface() {
				return true, errors.New(fmt.Sprintf("%v CanInterface false", fieldCache.Name))
			}
			// hash -> map
			err = cache.Get().GetMap(cacheKey, val.Interface())
			if cache.IsRedisError(err) {
				logger.Error("GetMap %v %v err:%v", cacheKey, cacheType, err)
				return true, err
			}
			return true, nil
		} else {
			logger.Error("%v unsupport cache type:%v", cacheKey, cacheType)
			return true, errors.New(fmt.Sprintf("%v unsupport cache type:%v", cacheKey, cacheType))
		}
	} else {
		for _, fieldCache := range structCache.Fields {
			val := reflectVal.Field(fieldCache.FieldIndex)
			if val.IsNil() {
				if !val.CanSet() {
					logger.Error("%v CanSet false", fieldCache.Name)
					return true, errors.New(fmt.Sprintf("%v CanSet false", fieldCache.Name))
				}
				newElem := reflect.New(fieldCache.StructField.Type)
				val.Set(newElem)
				logger.Debug("cacheKey:%v new %v", cacheKey, fieldCache.Name)
			}
			fieldInterface := val.Interface()
			hasCache, err := LoadFromCache(fieldInterface, cacheKey+"."+fieldCache.Name)
			if !hasCache {
				continue
			}
			if err != nil {
				logger.Error("LoadFromCache %v error:%v", cacheKey, err.Error())
				continue
			}
		}
	}
	return true, nil
}

// Entity的变化数据保存到数据库
func SaveEntityChangedDataToDb(entityDb db.EntityDb, entity Entity, removeCacheAfterSaveDb bool) error {
	changedDatas := make(map[string]interface{})
	var saved []Saveable
	var delKeys []string
	entity.RangeComponent(func(component Component) bool {
		structCache := GetSaveableStruct(reflect.TypeOf(component))
		if structCache == nil {
			return true
		}
		if !structCache.IsCompositeSaveable {
			if saveable, ok := component.(Saveable); ok {
				// 如果某个组件数据没改变过,就无需保存
				if !saveable.IsChanged() {
					logger.Debug("%v ignore %v", entity.GetId(), component.GetName())
					return true
				}
				saveData, err := GetComponentSaveData(component)
				if err != nil {
					logger.Error("%v Save %v err:%v", entity.GetId(), component.GetName(), err.Error())
					return true
				}
				// 使用protobuf存mongodb时,mongodb默认会把字段名转成小写,因为protobuf没设置bson tag
				changedDatas[component.GetNameLower()] = saveData
				if removeCacheAfterSaveDb {
					delKeys = append(delKeys, GetEntityComponentCacheKey("p", entity.GetId(), component.GetName()))
				}
				saved = append(saved, saveable)
				logger.Debug("SaveDb %v %v", entity.GetId(), component.GetName())
			}
		} else {
			reflectVal := reflect.ValueOf(component).Elem()
			for _, fieldCache := range structCache.Fields {
				childName := component.GetNameLower() + "." + fieldCache.Name
				val := reflectVal.Field(fieldCache.FieldIndex)
				if val.IsNil() {
					changedDatas[childName] = nil
					continue
				}
				fieldInterface := val.Interface()
				if saveable, ok := fieldInterface.(Saveable); ok {
					// 如果某个组件数据没改变过,就无需保存
					if !saveable.IsChanged() {
						logger.Debug("%v ignore %v.%v", entity.GetId(), component.GetName(), childName)
						continue
					}
					childSaveData, err := GetSaveData(fieldInterface, childName)
					if err != nil {
						logger.Error("%v Save %v.%v err:%v", entity.GetId(), component.GetName(), childName, err.Error())
						continue
					}
					changedDatas[childName] = childSaveData
					if removeCacheAfterSaveDb {
						delKeys = append(delKeys, GetEntityComponentChildCacheKey("p", entity.GetId(), component.GetName(), fieldCache.Name))
					}
					saved = append(saved, saveable)
					logger.Debug("SaveDb %v %v.%v", entity.GetId(), component.GetName(), childName)
				}
			}
		}
		return true
	})
	if len(changedDatas) == 0 {
		logger.Debug("ignore unchange data %v", entity.GetId())
		return nil
	}
	saveDbErr := entityDb.SaveComponents(entity.GetId(), changedDatas)
	if saveDbErr != nil {
		logger.Error("SaveDb %v err:%v", entity.GetId(), saveDbErr)
		logger.Error("%v", changedDatas)
	} else {
		logger.Debug("SaveDb %v", entity.GetId())
	}
	if saveDbErr == nil {
		// 保存数据库成功后,重置修改标记
		for _, saveable := range saved {
			saveable.ResetChanged()
		}
		if len(delKeys) > 0 {
			// 保存数据库成功后,才删除缓存
			cache.Get().Del(delKeys...)
			logger.Debug("RemoveCache %v %v", entity.GetId(), delKeys)
		}
	}
	return saveDbErr
}

// 获取实体需要保存到数据库的数据
func GetEntitySaveData(entity Entity, componentDatas map[string]interface{}) {
	entity.RangeComponent(func(component Component) bool {
		structCache := GetSaveableStruct(reflect.TypeOf(component))
		if structCache == nil {
			return true
		}
		saveData, err := GetComponentSaveData(component)
		if err != nil {
			logger.Error("%v %v err:%v", entity.GetId(), component.GetName(), err.Error())
			return true
		}
		componentDatas[component.GetNameLower()] = saveData
		logger.Debug("GetEntitySaveData %v %v", entity.GetId(), component.GetName())
		return true
	})
}
