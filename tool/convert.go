package tool

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
)

func ConvertCsvLineToValue(valueType reflect.Type, row []string, columnNames []string, option *CsvOption) reflect.Value {
	valueElemType := valueType
	if valueType.Kind() == reflect.Ptr {
		valueElemType = valueType.Elem() // *pb.ItemCfg -> pb.ItemCfg
	}
	newObject := reflect.New(valueElemType) // 如new(pb.ItemCfg)
	newObjectElem := newObject.Elem()
	if valueType.Kind() == reflect.Struct {
		newObject = newObject.Elem() // *pb.ItemCfg -> pb.ItemCfg
	}
	for columnIndex := 0; columnIndex < len(columnNames); columnIndex++ {
		columnName := columnNames[columnIndex]
		fieldString := row[columnIndex]
		fieldVal := newObjectElem.FieldByName(columnName)
		if fieldVal.Kind() == reflect.Ptr { // 指针类型的字段,如 Name *string
			fieldObj := reflect.New(fieldVal.Type().Elem()) // 如new(string)
			fieldVal.Set(fieldObj)                          // 如 obj.Name = new(string)
			fieldVal = fieldObj.Elem()                      // 如 *(obj.Name)
		}
		ConvertStringToFieldValue(newObject, fieldVal, columnName, fieldString, option, false)
	}
	return newObject
}

// 字段赋值,根据字段的类型,把字符串转换成对应的值
func ConvertStringToFieldValue(object, fieldVal reflect.Value, columnName, fieldString string, option *CsvOption, isSubStruct bool) {
	if !fieldVal.IsValid() {
		slog.Debug("unknown column", "columnName", columnName)
		return
	}
	if !fieldVal.CanSet() {
		slog.Error("field cant set", "columnName", columnName)
		return
	}
	fieldConverter := option.GetConverterByColumnName(columnName)
	if fieldConverter != nil {
		// 列名注册的自定义的转换接口
		v := fieldConverter(object.Interface(), columnName, fieldString)
		fieldVal.Set(reflect.ValueOf(v))
	} else {
		var convertFieldToElem bool
		fieldConverter, convertFieldToElem = option.GetConverterByTypePtrOrStruct(fieldVal.Type())
		if fieldConverter != nil {
			// 类型注册的自定义的转换接口
			v := fieldConverter(object.Interface(), columnName, fieldString)
			if v == nil {
				slog.Debug("field parse error", "columnName", columnName, "fieldString", fieldString)
				return
			}
			if convertFieldToElem {
				fieldVal.Set(reflect.ValueOf(v).Elem())
			} else {
				fieldVal.Set(reflect.ValueOf(v))
			}
			return
		}
		// 常规类型
		switch fieldVal.Type().Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			fieldVal.SetInt(util.Atoi64(fieldString))

		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			fieldVal.SetUint(util.Atou(fieldString))

		case reflect.String:
			fieldVal.SetString(fieldString)

		case reflect.Float32:
			f32, err := strconv.ParseFloat(fieldString, 32)
			if err != nil {
				slog.Error("float64 convert error", "columnName", columnName, "fieldString", fieldString, "err", err)
				break
			}
			fieldVal.SetFloat(f32)

		case reflect.Float64:
			f64, err := strconv.ParseFloat(fieldString, 64)
			if err != nil {
				slog.Error("float64 convert error", "columnName", columnName, "fieldString", fieldString, "err", err)
				break
			}
			fieldVal.SetFloat(f64)

		case reflect.Bool:
			fieldVal.SetBool(strings.ToLower(fieldString) == "true" || fieldString == "1")

		case reflect.Struct:
			if isSubStruct {
				// csv只是简单的以分隔符来解析,无法支持多层结构
				slog.Error("not support sub struct of sub struct", "columnName", columnName, "fieldString", fieldString)
				return
			}
			// 如CfgId_1#Num_2
			fieldValues := strings.Split(fieldString, option.MapSeparator)
			for _, fieldValue := range fieldValues {
				kvStr := strings.SplitN(fieldValue, option.MapKVSeparator, 2)
				if len(kvStr) != 2 {
					slog.Error("fieldValue convert error", "columnName", columnName, "fieldString", fieldString, "fieldValue", fieldValue)
					continue
				}
				subFieldVal := fieldVal.FieldByName(kvStr[0])
				if !subFieldVal.IsValid() {
					slog.Error("fieldValue convert error", "columnName", columnName, "fieldName", kvStr[0], "fieldString", fieldString, "fieldValue", fieldValue)
					continue
				}
				//gentity.ConvertStringToRealType(subFieldVal.Type(), kvStr[1])
				if subFieldVal.Kind() == reflect.Ptr { // 指针类型的字段,如 Name *string
					fieldObj := reflect.New(subFieldVal.Type().Elem()) // 如new(string)
					subFieldVal.Set(fieldObj)                          // 如 obj.Name = new(string)
					subFieldVal = fieldObj.Elem()                      // 如 *(obj.Name)
				}
				ConvertStringToFieldValue(fieldVal, subFieldVal, kvStr[0], kvStr[1], option, true)
			}

		case reflect.Slice:
			// 常规数组解析
			if fieldString == "" {
				return
			}
			newSlice := reflect.MakeSlice(fieldVal.Type(), 0, 0)
			sliceElemType := fieldVal.Type().Elem()
			converter, convertToElem := option.GetConverterByTypePtrOrStruct(sliceElemType)
			if converter == nil {
				if sliceElemType.Kind() == reflect.Struct {
					convertToElem = true
				} else if sliceElemType.Kind() == reflect.Ptr && sliceElemType.Elem().Kind() == reflect.Struct {
					sliceElemType = sliceElemType.Elem()
				}
			}
			sArray := strings.Split(fieldString, option.SliceSeparator)
			for _, str := range sArray {
				if str == "" {
					continue
				}
				var sliceElemValue any
				if converter != nil {
					sliceElemValue = converter(object.Interface(), columnName, str)
				} else {
					if sliceElemType.Kind() == reflect.Struct {
						fieldObj := reflect.New(sliceElemType) // 如obj := new(Struct)
						sliceElemValue = fieldObj.Interface()
						subFieldVal := fieldObj.Elem() // 如 *(obj)
						// 数组支持子结构
						ConvertStringToFieldValue(fieldVal, subFieldVal, "", str, option, isSubStruct)
					} else {
						sliceElemValue = gentity.ConvertStringToRealType(sliceElemType, str)
					}
				}
				if sliceElemValue == nil {
					slog.Error("slice item parse error", "columnName", columnName, "fieldString", fieldString, "str", str)
					continue
				}
				if convertToElem {
					newSlice = reflect.Append(newSlice, reflect.ValueOf(sliceElemValue).Elem())
				} else {
					newSlice = reflect.Append(newSlice, reflect.ValueOf(sliceElemValue))
				}
			}
			fieldVal.Set(newSlice)

		case reflect.Map:
			// 常规map解析
			if fieldString == "" {
				return
			}
			newMap := reflect.MakeMap(fieldVal.Type())
			fieldKeyType := fieldVal.Type().Key()
			fieldValueType := fieldVal.Type().Elem()
			converter, convertToElem := option.GetConverterByTypePtrOrStruct(fieldValueType)
			mapItemStrs := strings.Split(fieldString, option.MapSeparator)
			for _, mapItemStr := range mapItemStrs {
				kvStr := strings.SplitN(mapItemStr, option.MapKVSeparator, 2)
				if len(kvStr) == 2 {
					fieldKeyValue := gentity.ConvertStringToRealType(fieldKeyType, kvStr[0])
					var fieldValueValue any
					if converter != nil {
						fieldValueValue = converter(object.Interface(), columnName, kvStr[1])
					} else {
						// NOTE: map不支持子结构,分隔符冲突了
						fieldValueValue = gentity.ConvertStringToRealType(fieldValueType, kvStr[1])
					}
					if fieldValueValue == nil {
						slog.Error("map value parse error", "columnName", columnName, "fieldString", fieldString, "kvStr", kvStr)
						continue
					}
					if convertToElem {
						newMap.SetMapIndex(reflect.ValueOf(fieldKeyValue), reflect.ValueOf(fieldValueValue).Elem())
					} else {
						newMap.SetMapIndex(reflect.ValueOf(fieldKeyValue), reflect.ValueOf(fieldValueValue))
					}
				} else {
					slog.Error("map kv len error", "columnName", columnName, "fieldString", fieldString, "kvStr", kvStr)
				}
			}
			fieldVal.Set(newMap)

		default:
			slog.Error("unsupported kind", "columnName", columnName, "fieldVal", fieldVal, "kind", fieldVal.Type().Kind())
			return
		}
	}
}

//// Type_2#Total_5#Properties_{IsPvp_true&IsWin_true}
//func parseStructString(structString string, option *CsvOption) map[string]string {
//	s := structString
//	//leftBracket := strings.Index(s, "_{")
//	//if leftBracket > 0 {
//	//	s = s[leftBracket+2:]
//	//	rightBracket := strings.Index(s, "}")
//	//}
//	m := make(map[string]string)
//	i := 0
//	begin := 0
//	n := len(structString)
//	for i < n {
//		end := strings.Index(s, "#")
//		subBegin := strings.Index(s, "_{")
//		if end < 0 {
//			kvPairString := s[begin:n]
//			kvPair := strings.Split(kvPairString, "_")
//			if len(kvPair) == 2 {
//				m[kvPair[0]] = kvPair[1]
//			}
//			break
//		} else {
//			kvPairString := s[begin:end]
//			kvPair := strings.Split(kvPairString, "_")
//			if len(kvPair) == 2 {
//				m[kvPair[0]] = kvPair[1]
//			}
//		}
//		s = s[end+1:]
//		i = 0
//		n = len(s)
//	}
//	return m
//}
