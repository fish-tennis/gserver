package tool

import (
	"encoding/csv"
	"errors"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	"log/slog"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type FieldConverter func(obj interface{}, columnName, fieldStr string) interface{}

type CsvOption struct {
	// 数据行索引(>=1)
	DataBeginRowIndex int

	// 数组分隔符
	SliceSeparator string

	// Map的kv分隔符
	MapKVSeparator string
	// Map分隔符
	MapSeparator string

	// 自定义转换函数
	// 把csv的字符串转换成其他对象
	customFieldConverters map[string]FieldConverter
}

func NewCsvOption(dataBeginRowIndex int) *CsvOption {
	return &CsvOption{
		DataBeginRowIndex:     dataBeginRowIndex,
		customFieldConverters: make(map[string]FieldConverter),
	}
}

func (co *CsvOption) AddFieldConverter(fieldName string, converter FieldConverter) *CsvOption {
	if co.customFieldConverters == nil {
		co.customFieldConverters = make(map[string]FieldConverter)
	}
	co.customFieldConverters[fieldName] = converter
	return co
}

func (co *CsvOption) GetFieldConverter(fieldName string) FieldConverter {
	if co.customFieldConverters == nil {
		return nil
	}
	return co.customFieldConverters[fieldName]
}

type IntOrString interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~string
}

// V支持proto.Message和普通struct结构
func ReadCsvFile[M ~map[K]V, K IntOrString, V any](file string, m M, option *CsvOption) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	rows, readErr := csv.NewReader(f).ReadAll()
	if readErr != nil {
		return readErr
	}
	return ReadCsvFromData(rows, m, option)
}

// V支持proto.Message和普通struct结构
func ReadCsvFromData[M ~map[K]V, K IntOrString, V any](rows [][]string, m M, option *CsvOption) error {
	if len(rows) == 0 {
		return errors.New("no csv header")
	}
	columnNames := rows[0]
	if len(columnNames) == 0 {
		return errors.New("no column")
	}
	if option.DataBeginRowIndex < 1 {
		return errors.New("DataBeginRowIndex must >=1")
	}
	mType := reflect.TypeOf(m)
	mVal := reflect.ValueOf(m)
	keyType := mType.Key()    // key type of m, 如int
	valueType := mType.Elem() // value type of m, 如*pb.ItemCfg
	if valueType.Kind() != reflect.Ptr {
		return errors.New("valueType must be Ptr")
	}
	slog.Debug("types", "mType", mType.Kind(), "mVal", mVal.Kind(),
		"keyType", keyType, "valueType", valueType)
	for rowIndex := option.DataBeginRowIndex; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex]
		// 固定第一列是key
		key := gentity.ConvertStringToRealType(keyType, row[0])

		newItem := reflect.New(valueType.Elem()) // 如new(pb.ItemCfg)
		newItemElem := newItem.Elem()
		slog.Debug("newItem", "newItem", newItem.Kind(), "newItemElem", newItemElem.Kind())
		for columnIndex := 0; columnIndex < len(columnNames); columnIndex++ {
			fieldName := columnNames[columnIndex]
			fieldString := row[columnIndex]
			fieldVal := newItemElem.FieldByName(fieldName)
			if !fieldVal.IsValid() {
				slog.Error("fieldVal error:", "fieldName", fieldName)
				continue
			}
			if !fieldVal.CanSet() {
				slog.Error("fieldVal cant set:", "fieldName", fieldName)
				continue
			}
			fieldConverter := option.GetFieldConverter(fieldName)
			if fieldConverter != nil {
				// 自定义的转换接口
				v := fieldConverter(newItem.Interface(), fieldName, fieldString)
				fieldVal.Set(reflect.ValueOf(v))
			} else {
				switch fieldVal.Type().Kind() {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					fieldVal.SetInt(util.Atoi64(fieldString))
					slog.Debug("SetInt", fieldName, fieldVal.Int())
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					fieldVal.SetUint(util.Atou(fieldString))
					slog.Debug("SetUint", fieldName, fieldVal.Uint())
				case reflect.String:
					fieldVal.SetString(fieldString)
					slog.Debug("SetString", fieldName, fieldVal.String())
				case reflect.Float64:
					f64, err := strconv.ParseFloat(fieldString, 64)
					if err != nil {
						slog.Debug("float64 convert error:", "fieldName", fieldName, "fieldString", fieldString, "err", err)
						break
					}
					fieldVal.SetFloat(f64)
					slog.Debug("SetFloat", fieldName, fieldVal.Float())
				case reflect.Bool:
					fieldVal.SetBool(fieldString == "true" || fieldString == "1")
					slog.Debug("SetBool", fieldName, fieldVal.Bool())
				case reflect.Slice:
					// 常规数组解析
					newSlice := reflect.MakeSlice(fieldVal.Type(), 0, 0)
					sliceElemType := fieldVal.Type().Elem()
					strs := strings.Split(fieldString, option.SliceSeparator)
					for _, str := range strs {
						sliceElemValue := gentity.ConvertStringToRealType(sliceElemType, str)
						newSlice = reflect.Append(newSlice, reflect.ValueOf(sliceElemValue))
					}
					fieldVal.Set(newSlice)
				case reflect.Map:
					// 常规map解析
					newMap := reflect.MakeMap(fieldVal.Type())
					fieldKeyType := fieldVal.Type().Key()
					fieldValueType := fieldVal.Type().Elem()
					mapItemStrs := strings.Split(fieldString, option.MapSeparator)
					for _, mapItemStr := range mapItemStrs {
						kvStr := strings.Split(mapItemStr, option.MapKVSeparator)
						if len(kvStr) == 2 {
							fieldKeyValue := gentity.ConvertStringToRealType(fieldKeyType, kvStr[0])
							fieldValueValue := gentity.ConvertStringToRealType(fieldValueType, kvStr[1])
							newMap.SetMapIndex(reflect.ValueOf(fieldKeyValue), reflect.ValueOf(fieldValueValue))
						}
					}
					fieldVal.Set(newMap)
				default:
					slog.Debug("unsupported kind:", "fieldName", fieldName, "fieldVal", fieldVal, "kind", fieldVal.Type().Kind())
					continue
				}
			}

		}
		// 下面的代码会导致slog停止打印后续的日志,why?
		//slog.Debug("parse row", "key", key, "newItem", fmt.Sprintf("%v", newItemIf))
		mVal.SetMapIndex(reflect.ValueOf(key), reflect.ValueOf(newItem.Interface()))
	}
	return nil
}
