package internal

import (
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/util"
	"reflect"
	"strings"
	"sync"
)

var _saveableStructsMap = newSaveableStructsMap()

// 有需要保存字段的结构
type SaveableStruct struct {
	// 单个db字段
	Field *SaveableField
	// 多个child字段
	Children []*SaveableField
}

// 是否是单个db字段
func (this *SaveableStruct) IsSingleField() bool {
	return this.Field != nil
}

// 字段
type SaveableField struct {
	StructField reflect.StructField
	FieldIndex  int
	// 是否明文保存
	IsPlain     bool
	// 保存的字段名
	Name        string
}

type safeSaveableStructsMap struct {
	// 是否使用全小写
	useLowerName bool
	m map[reflect.Type]*SaveableStruct
	// 如果在初始化的时候把所有结构缓存的话,这个读写锁是可以去掉的
	l *sync.RWMutex
}

func (s *safeSaveableStructsMap) Set(key reflect.Type, value *SaveableStruct) {
	s.l.Lock()
	defer s.l.Unlock()
	s.m[key] = value
	if value != nil {
		logger.Debug("SaveableStruct: %v", key)
	}
}

func (s *safeSaveableStructsMap) Get(key reflect.Type) (*SaveableStruct,bool) {
	s.l.RLock()
	defer s.l.RUnlock()
	v,ok := s.m[key]
	return v,ok
}

func newSaveableStructsMap() *safeSaveableStructsMap {
	return &safeSaveableStructsMap{
		useLowerName: true, // 默认使用全小写
		l: new(sync.RWMutex),
		m: make(map[reflect.Type]*SaveableStruct),
	}
}

func GetSaveableStruct(reflectType reflect.Type) *SaveableStruct {
	if reflectType.Kind() == reflect.Ptr {
		reflectType = reflectType.Elem()
	}
	if reflectType.Kind() != reflect.Struct {
		return nil
	}
	cacheStruct,ok := _saveableStructsMap.Get(reflectType)
	if ok {
		return cacheStruct
	}
	structCahce := &SaveableStruct{}
	// 检查db字段
	for i := 0; i < reflectType.NumField(); i++ {
		fieldStruct := reflectType.Field(i)
		if len(fieldStruct.Tag) == 0 {
			continue
		}
		isPlain := false
		dbSetting,ok := fieldStruct.Tag.Lookup("db")
		if !ok {
			continue
		}
		// db字段只能有一个
		if structCahce.Field != nil {
			logger.Error("%v.%v db field count error", reflectType.Name(), fieldStruct.Name)
			continue
		}
		dbSettings := strings.Split(dbSetting, ";")
		if util.HasString(dbSettings, "plain") {
			isPlain = true
		}
		// 默认使用字段名的全小写
		name := strings.ToLower(fieldStruct.Name)
		for _,n := range dbSettings {
			if n != "" && n != "plain" {
				// 自动转全小写
				if _saveableStructsMap.useLowerName {
					name = strings.ToLower(n)
				} else {
					name = n
				}
				break
			}
		}
		fieldCache := &SaveableField{
			StructField: fieldStruct,
			FieldIndex:  i,
			IsPlain:     isPlain,
			Name:        name,
		}
		structCahce.Field = fieldCache
		logger.Debug("db %v.%v plain:%v", reflectType.Name(), name, isPlain)
	}
	structCahce.Children = make([]*SaveableField, 0)
	// 检查child字段
	for i := 0; i < reflectType.NumField(); i++ {
		fieldStruct := reflectType.Field(i)
		if len(fieldStruct.Tag) == 0 {
			continue
		}
		dbSetting,ok := fieldStruct.Tag.Lookup("child")
		if !ok {
			continue
		}
		// db字段和child字段不共存
		if structCahce.Field != nil {
			logger.Error("%v already have db field,%v cant work", reflectType.Name(), fieldStruct.Name)
			continue
		}
		// 默认使用字段名的全小写
		name := strings.ToLower(fieldStruct.Name)
		dbSettings := strings.Split(dbSetting, ";")
		for _,n := range dbSettings {
			if n != "" {
				// 自动转全小写
				if _saveableStructsMap.useLowerName {
					name = strings.ToLower(n)
				} else {
					name = n
				}
				break
			}
		}
		fieldCache := &SaveableField{
			StructField: fieldStruct,
			FieldIndex:  i,
			Name:        name,
		}
		structCahce.Children = append(structCahce.Children, fieldCache)
		logger.Debug("child %v.%v", reflectType.Name(), name)
		GetSaveableStruct(fieldCache.StructField.Type)
	}
	if structCahce.Field == nil && len(structCahce.Children) == 0 {
		_saveableStructsMap.Set(reflectType, nil)
		return nil
	}
	_saveableStructsMap.Set(reflectType, structCahce)
	return structCahce
}