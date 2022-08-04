package internal

import (
	"github.com/fish-tennis/gserver/util"
	"reflect"
	"strings"
)

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

func GetSaveableStruct(reflectType reflect.Type) *SaveableStruct {
	if reflectType.Kind() == reflect.Ptr {
		reflectType = reflectType.Elem()
	}
	if reflectType.Kind() != reflect.Struct {
		return nil
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
		dbSetting := fieldStruct.Tag.Get("db")
		if len(dbSetting) == 0 {
			continue
		}
		dbSettings := strings.Split(dbSetting, ";")
		if util.HasString(dbSettings, "plain") {
			isPlain = true
		}
		name := strings.ToLower(fieldStruct.Name)
		for _,n := range dbSettings {
			if n != "" && n != "plain" {
				name = n
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
		//logger.Debug("%v %v %v", reflectType.Name(), name, isPlain)
	}
	if len(structCahce.Fields) > 0 {
		return structCahce
	}
	for i := 0; i < reflectType.NumField(); i++ {
		fieldStruct := reflectType.Field(i)
		if len(fieldStruct.Tag) == 0 {
			continue
		}
		dbSetting := fieldStruct.Tag.Get("child")
		if len(dbSetting) == 0 {
			continue
		}
		structCahce.IsCompositeSaveable = true
		name := strings.ToLower(fieldStruct.Name)
		dbSettings := strings.Split(dbSetting, ";")
		for _,n := range dbSettings {
			if n != "" {
				name = n
				break
			}
		}
		fieldCache := &SaveableField{
			StructField: fieldStruct,
			FieldIndex:  i,
			Name:        name,
		}
		structCahce.Fields = append(structCahce.Fields, fieldCache)
		//logger.Debug("%v.%v", reflectType.Name(), name)
	}
	if len(structCahce.Fields) == 0 {
		return nil
	}
	return structCahce
}