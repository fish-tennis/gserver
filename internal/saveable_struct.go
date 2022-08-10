package internal

import (
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/util"
	"reflect"
	"strings"
	"sync"
)

var _saveableStructsMap = newSaveableStructsMap()

type SaveableStruct struct {
	// 是否是子模块的组合
	IsCompositeSaveable bool
	// 字段
	Fields []*SaveableField
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
	structCahce := &SaveableStruct{
		Fields: make([]*SaveableField, 0),
	}
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
		structCahce.Fields = append(structCahce.Fields, fieldCache)
		logger.Debug("%v %v %v", reflectType.Name(), name, isPlain)
	}
	if len(structCahce.Fields) > 0 {
		_saveableStructsMap.Set(reflectType, structCahce)
		return structCahce
	}
	for i := 0; i < reflectType.NumField(); i++ {
		fieldStruct := reflectType.Field(i)
		if len(fieldStruct.Tag) == 0 {
			continue
		}
		dbSetting,ok := fieldStruct.Tag.Lookup("child")
		if !ok {
			continue
		}
		structCahce.IsCompositeSaveable = true
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
		structCahce.Fields = append(structCahce.Fields, fieldCache)
		logger.Debug("child %v.%v", reflectType.Name(), name)
	}
	if len(structCahce.Fields) == 0 {
		_saveableStructsMap.Set(reflectType, nil)
		return nil
	}
	_saveableStructsMap.Set(reflectType, structCahce)
	return structCahce
}