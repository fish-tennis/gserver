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
	isDirty bool
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
	dirtyMap map[string]bool
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

// 保存数据库之前,对Saveable进行预处理
func SaveSaveable(saveable Saveable) (interface{},error) {
	saveData,protoMarshal := saveable.DbData()
	if protoMarshal {
		// 模块的保存数据是一个proto.Message
		// proto.Message -> []byte
		if protoMessage,ok := saveData.(proto.Message); ok {
			return proto.Marshal(protoMessage)
		}
		val := reflect.ValueOf(saveData)
		switch val.Kind() {
		case reflect.Map:
			// 模块的保存数据是一个map
			typ := reflect.TypeOf(saveData)
			keyType := typ.Key()
			valType := typ.Elem()
			if valType.Kind() == reflect.Interface || valType.Kind() == reflect.Ptr {
				switch keyType.Kind() {
				case reflect.Int,reflect.Int8,reflect.Int16,reflect.Int32,reflect.Int64:
					// map[int]proto.Message -> map[int64][]byte
					// map[int]interface{} -> map[int64]interface{}
					newMap := make(map[int64]interface{})
					it := val.MapRange()
					for it.Next() {
						// map的value是proto格式,进行序列化
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
					// map[uint]proto.Message -> map[uint64][]byte
					// map[uint]interface{} -> map[uint64]interface{}
					newMap := make(map[uint64]interface{})
					it := val.MapRange()
					for it.Next() {
						// map的value是proto格式,进行序列化
						if protoMessage,ok := it.Value().Interface().(proto.Message); ok {
							bytes,err := proto.Marshal(protoMessage)
							if err != nil {
								logger.Error("%v proto %v err:%v", saveable.GetCacheKey(), it.Key().Uint(), err.Error())
								return nil, err
							}
							newMap[it.Key().Uint()] = bytes
						} else {
							newMap[it.Key().Uint()] = it.Value().Interface()
						}
					}
					return newMap,nil
				case reflect.String:
					// map[string]proto.Message -> map[string][]byte
					// map[string]interface{} -> map[string]interface{}
					newMap := make(map[string]interface{})
					it := val.MapRange()
					for it.Next() {
						// map的value是proto格式,进行序列化
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
					logger.Error("%v unsupport key type:%v", saveable.GetCacheKey(), keyType.Kind())
					return nil, errors.New("unsupport key type")
				}
			} else {
				return saveData,nil
			}
		}
	} else {
		switch realData := saveData.(type) {
		case *SliceInt32:
			// SliceInt32 -> []int32
			return realData.Data(),nil
		}
	}
	return saveData,nil
}

// 加载数据
func LoadSaveable(saveable Saveable, sourceData interface{}) error {
	if util.IsNil(sourceData) {
		return nil
	}
	dbData,protoMarshal := saveable.DbData()
	if util.IsNil(dbData) {
		return nil
	}
	typ := reflect.TypeOf(dbData)
	sourceTyp := reflect.TypeOf(sourceData)
	switch sourceTyp.Kind() {
	case reflect.Slice:
		if protoMarshal {
			// []byte -> proto.Message
			if sourceTyp.Elem().Kind() == reflect.Uint8 {
				if bytes,ok := sourceData.([]byte); ok {
					if len(bytes) == 0 {
						return nil
					}
					// []byte -> proto.Message
					if protoMessage,ok2 := dbData.(proto.Message); ok2 {
						err := proto.Unmarshal(bytes, protoMessage)
						if err != nil {
							logger.Error("%v proto err:%v", saveable.GetCacheKey(), err.Error())
							return err
						}
						return nil
					}
				}
			}
		}
		switch realData := dbData.(type) {
		case *SliceInt32:
			// []int -> SliceInt32
			sourceVal := reflect.ValueOf(sourceData)
			sourceValType := sourceTyp.Elem()
			for i := 0; i < sourceVal.Len(); i++ {
				realData.Append(int32(convertValueToInt(sourceValType, sourceVal.Index(i))))
			}
			logger.Debug("%v slice:%v", saveable.GetCacheKey(), realData.Data())
			return nil
		default:
			logger.Error("%v unsupport type:%v", saveable.GetCacheKey(), sourceTyp.Kind())
			return errors.New(fmt.Sprintf("%v unsupport type:%v", saveable.GetCacheKey(), sourceTyp.Kind()))
		}

	case reflect.Map:
		// map[intORstring]intORstring -> map[intORstring]intORstring
		// map[intORstring][]byte -> map[intORstring]proto.Message
		sourceVal := reflect.ValueOf(sourceData)
		sourceKeyType := sourceTyp.Key()
		sourceValType := sourceTyp.Elem()
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
	logger.Error("%v unsupport type:%v", saveable.GetCacheKey(), sourceTyp.Kind())
	return errors.New(fmt.Sprintf("%v unsupport type:%v", saveable.GetCacheKey(), sourceTyp.Kind()))
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
		// mongodb中读出来是proto.Message格式,转换成map[string]interface{}
		// map的key对应ChildSaveable的Key()接口
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
		// 子模块对应的数据
		v := sourceVal.MapIndex(reflect.ValueOf(saveable.Key()))
		if !v.IsValid() {
			logger.Debug("saveable not exists:%v", saveable.GetCacheKey())
			continue
		}
		switch v.Kind() {
		case reflect.Slice:
			if v.Type().Elem().Kind() != reflect.Uint8 {
				logger.Error("%v unsupport slice type:%v", saveable.GetCacheKey(), v.Type().Elem().Kind())
				return errors.New(fmt.Sprintf("unsupport slice type:%v",v.Type().Elem().Kind()))
			}
			err := LoadSaveable(saveable, v.Bytes())
			if err != nil {
				return err
			}
		case reflect.Interface,reflect.Ptr:
			err := LoadSaveable(saveable, v.Interface())
			if err != nil {
				return err
			}
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
	case reflect.Float32,reflect.Float64:
		return convertInterfaceToRealType(dstType, v.Float())
	case reflect.String:
		return convertInterfaceToRealType(dstType, v.String())
	case reflect.Interface,reflect.Ptr:
		return convertInterfaceToRealType(dstType, v.Interface())
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return convertInterfaceToRealType(dstType, v.Bytes())
		} else {
			return convertInterfaceToRealType(dstType, v.Interface())
		}
	}
	logger.Error("unsupport type:%v",srcType.Kind())
	return nil
}

// reflect.Value -> int
func convertValueToInt(srcType reflect.Type, v reflect.Value) int64 {
	switch srcType.Kind() {
	case reflect.Int,reflect.Int8,reflect.Int16,reflect.Int32,reflect.Int64:
		return v.Int()
	case reflect.Uint,reflect.Uint8,reflect.Uint16,reflect.Uint32,reflect.Uint64:
		return int64(v.Uint())
	case reflect.Float32,reflect.Float64:
		// NOTE:有精度问题
		return int64(v.Float())
	}
	logger.Error("unsupport type:%v",srcType.Kind())
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
	case reflect.Ptr,reflect.Slice:
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
			} else {
				v = fieldVal.Interface()
			}
		case reflect.Interface,reflect.Ptr,reflect.Map:
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

// 把修改数据保存到缓存
func SaveDirtyCache(data interface{}) {
	if saveable,ok := data.(Saveable); ok {
		// 缓存数据作为一个整体的
		SaveSaveableDirtyCache(saveable)
	}
	if compositeSaveable,ok := data.(CompositeSaveable); ok {
		SaveCompositeSaveableDirtyCache(compositeSaveable)
	}
}

// 把修改数据保存到缓存
func SaveSaveableDirtyCache(saveable Saveable) {
	// 缓存数据作为一个整体的
	if dirtyMark,ok2 := saveable.(DirtyMark); ok2 {
		if !dirtyMark.IsDirty() {
			return
		}
		cacheData := saveable.CacheData()
		if cacheData == nil {
			return
		}
		cacheKeyName := saveable.GetCacheKey()
		if reflect.ValueOf(cacheData).Kind() == reflect.Map {
			// map格式作为一个整体缓存时,需要先删除之前的数据
			err := cache.Get().Del(cacheKeyName)
			if cache.IsRedisError(err) {
				logger.Error("%v cache err:%v", cacheKeyName, err.Error())
				return
			}
			err = cache.Get().SetMap(cacheKeyName, cacheData)
			if cache.IsRedisError(err) {
				logger.Error("%v cache err:%v", cacheKeyName, err.Error())
				return
			}
		} else {
			switch realData := cacheData.(type) {
			case proto.Message:
				// proto.Message -> []byte
				err := cache.Get().SetProto(cacheKeyName, realData, 0)
				if cache.IsRedisError(err) {
					logger.Error("%v cache err:%v", cacheKeyName, err.Error())
					return
				}
			case *SliceInt32:
				// SliceInt32 -> string
				err := cache.Get().Set(cacheKeyName, realData.ToString(), 0)
				if cache.IsRedisError(err) {
					logger.Error("%v cache err:%v", cacheKeyName, err.Error())
					return
				}
			default:
				logger.Error("%v cache err:unsupport type", cacheKeyName)
				return
			}
		}
		dirtyMark.ResetDirty()
		logger.Debug("SaveCache %v", cacheKeyName)
		return
	}
	// map格式的
	if dirtyMark,ok2 := saveable.(MapDirtyMark); ok2 {
		if !dirtyMark.IsDirty() {
			return
		}
		cacheKeyName := saveable.GetCacheKey()
		if !dirtyMark.HasCached() {
			// 必须把整体数据缓存一次,后面的修改才能增量更新
			cacheData := saveable.CacheData()
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
			for dirtyKey,isAddOrUpdate := range dirtyMark.GetDirtyMap() {
				if isAddOrUpdate {
					if dirtyValue,exists := dirtyMark.GetMapValue(dirtyKey); exists {
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
		dirtyMark.ResetDirty()
		logger.Debug("SaveCache %v", cacheKeyName)
		return
	}
}

// 把组合模块的修改数据保存到缓存
func SaveCompositeSaveableDirtyCache(compositeSaveable CompositeSaveable) {
	saveables := compositeSaveable.SaveableChildren()
	for _,saveable := range saveables {
		SaveSaveableDirtyCache(saveable)
	}
}

// 从缓存中恢复数据
func LoadFromCache(saveable Saveable) (bool,error) {
	cacheKey := saveable.GetCacheKey()
	cacheType,err := cache.Get().Type(cacheKey)
	if err == redis.Nil || cacheType == "" || cacheType == "none" {
		return false,nil
	}
	cacheData := saveable.CacheData()
	if cacheType == "string" {
		switch realData := cacheData.(type) {
		case proto.Message:
			// []byte -> proto.Message
			err = cache.Get().GetProto(cacheKey, realData)
			if cache.IsRedisError(err) {
				logger.Error("GetProto %v %v err:%v", cacheKey, cacheType, err)
				return true,err
			}
			return true,nil
		case *SliceInt32:
			// string -> SliceInt32
			strData,err := cache.Get().Get(cacheKey)
			if cache.IsRedisError(err) {
				logger.Error("GetProto %v %v err:%v", cacheKey, cacheType, err)
				return true,err
			}
			realData.FromString(strData)
			return true,nil
		default:
			logger.Error("%v unsupport cache type:%v", cacheKey, cacheType)
			return true,errors.New(fmt.Sprintf("%v unsupport cache type:%v", cacheKey, cacheType))
		}
	} else if cacheType == "hash" {
		// hash -> map
		err = cache.Get().GetMap(cacheKey, cacheData)
		if cache.IsRedisError(err) {
			logger.Error("GetMap %v %v err:%v", cacheKey, cacheType, err)
			return true,err
		}
		return true,nil
	} else {
		logger.Error("%v unsupport cache type:%v", cacheKey, cacheType)
		return true,errors.New(fmt.Sprintf("%v unsupport cache type:%v", cacheKey, cacheType))
	}
	return true,nil
}

// Entity数据保存到数据库
func SaveEntityToDb(entityDb db.EntityDb, entity Entity, removeCacheAfterSaveDb bool) error {
	componentDatas := make(map[string]interface{})
	var saved []Saveable
	var delKeys []string
	entity.RangeComponent(func(component Component) bool {
		if saveable, ok := component.(Saveable); ok {
			// 如果某个组件数据没改变过,就无需保存
			if !saveable.IsChanged() {
				logger.Debug("%v ignore %v", entity.GetId(), component.GetName())
				return true
			}
			saveData, err := SaveSaveable(saveable)
			if err != nil {
				logger.Error("%v Save %v err:%v", entity.GetId(), component.GetName(), err.Error())
				return true
			}
			if saveData == nil {
				logger.Debug("%v ignore nil %v", entity.GetId(), component.GetName())
				return true
			}
			// 使用protobuf存mongodb时,mongodb默认会把字段名转成小写,因为protobuf没设置bson tag
			componentDatas[component.GetNameLower()] = saveData
			if removeCacheAfterSaveDb {
				delKeys = append(delKeys, saveable.GetCacheKey())
			}
			saved = append(saved, saveable)
			logger.Debug("SaveDb %v %v", entity.GetId(), component.GetName())
		}
		if compositeSaveable, ok := component.(CompositeSaveable); ok {
			//compositeData := make(map[string]interface{})
			saveables := compositeSaveable.SaveableChildren()
			saveChildCount := 0
			// 只需要保存修改过数据的子模块
			for _, saveable := range saveables {
				if !saveable.IsChanged() {
					logger.Debug("%v ignore %v", entity.GetId(), saveable.GetCacheKey())
					continue
				}
				saveData, err := SaveSaveable(saveable)
				if err != nil {
					logger.Error("%v Save %v err:%v", entity.GetId(), saveable.GetCacheKey(), err.Error())
					continue
				}
				if saveData == nil {
					logger.Debug("%v ignore nil %v", entity.GetId(), component.GetName())
					continue
				}
				componentDatas[component.GetNameLower()+"."+saveable.Key()] = saveData
				saveChildCount++
				if removeCacheAfterSaveDb {
					delKeys = append(delKeys, saveable.GetCacheKey())
				}
				saved = append(saved, saveable)
				logger.Debug("SaveDb %v %v.%v", entity.GetId(), component.GetNameLower(), saveable.Key())
			}
			if saveChildCount > 0 {
				//componentDatas[component.GetNameLower()] = compositeData
				logger.Debug("SaveDb %v %v child:%v", entity.GetId(), component.GetName(), saveChildCount)
			}
		}
		return true
	})
	if len(componentDatas) == 0 {
		logger.Debug("ignore unchange data %v", entity.GetId())
		return nil
	}
	saveDbErr := entityDb.SaveComponents(entity.GetId(), componentDatas)
	if saveDbErr != nil {
		logger.Error("SaveDb %v err:%v", entity.GetId(), saveDbErr)
	} else {
		logger.Debug("SaveDb %v", entity.GetId())
	}
	if saveDbErr == nil {
		for _,saveable := range saved {
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