package tool

import (
	"encoding/csv"
	"errors"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	"log/slog"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// 字段转换接口
type FieldConverter func(obj interface{}, columnName, fieldStr string) interface{}

type CsvOption struct {
	// 数据行索引(>=1)
	DataBeginRowIndex int

	// 数组分隔符
	// 如数组分隔符为;时,则1;2;3可以表示[1,2,3]的数组
	SliceSeparator string

	// Map的分隔符
	// 如MapKVSeparator为_ MapSeparator为;
	// 则a_1;b_2;c_3可以表示{"a":1,"b":2,"c":3}的map
	MapKVSeparator string
	MapSeparator   string

	// 自定义转换函数
	// 把csv的字符串转换成其他对象 以列名作为关键字
	customFieldConvertersByColumnName map[string]FieldConverter
	// 把csv的字符串转换成其他对象 以字段类型作为关键字
	customFieldConvertersByType map[reflect.Type]FieldConverter
}

// 注册列名对应的转换接口
func (co *CsvOption) RegisterConverterByColumnName(columnName string, converter FieldConverter) *CsvOption {
	if co.customFieldConvertersByColumnName == nil {
		co.customFieldConvertersByColumnName = make(map[string]FieldConverter)
	}
	co.customFieldConvertersByColumnName[columnName] = converter
	return co
}

func (co *CsvOption) GetConverterByColumnName(columnName string) FieldConverter {
	if co.customFieldConvertersByColumnName == nil {
		return nil
	}
	return co.customFieldConvertersByColumnName[columnName]
}

// 注册类型对应的转换接口
func (co *CsvOption) RegisterConverterByType(typ reflect.Type, converter FieldConverter) *CsvOption {
	if co.customFieldConvertersByType == nil {
		co.customFieldConvertersByType = make(map[reflect.Type]FieldConverter)
	}
	co.customFieldConvertersByType[typ] = converter
	return co
}

func (co *CsvOption) GetConverterByType(typ reflect.Type) FieldConverter {
	if co.customFieldConvertersByType == nil {
		return nil
	}
	return co.customFieldConvertersByType[typ]
}

// 如果typ是Struct,但是注册的FieldConverter是同类型的Ptr,则会返回Ptr类型的FieldConverter,同时convertToElem返回true
func (co *CsvOption) GetConverterByTypePtrOrStruct(typ reflect.Type) (converter FieldConverter, convertToElem bool) {
	if co.customFieldConvertersByType == nil {
		return
	}
	converter, _ = co.customFieldConvertersByType[typ]
	if converter == nil {
		if typ.Kind() == reflect.Struct {
			converter = co.GetConverterByType(reflect.PtrTo(typ))
			// 注册的是指针类型,转换后,需要把ptr转换成elem
			convertToElem = converter != nil
			return
		}
		//else if typ.Kind() == reflect.Ptr {
		//	converter = co.GetConverterByType(typ.Elem())
		//	convertToPtr = converter != nil
		//	return
		//}
	}
	return
}

type IntOrString interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~string
}

// csv数据转换成map
// V支持proto.Message和普通struct结构
func ReadCsvFileMap[M ~map[K]V, K IntOrString, V any](file string, m M, option *CsvOption) error {
	rows, readErr := ReadCsvFile(file)
	if readErr != nil {
		return readErr
	}
	return ReadCsvFromDataMap(rows, m, option)
}

// csv数据转换成slice
// V支持proto.Message和普通struct结构
func ReadCsvFileSlice[Slice ~[]V, V any](file string, s Slice, option *CsvOption) (Slice, error) {
	rows, readErr := ReadCsvFile(file)
	if readErr != nil {
		return s, readErr
	}
	return ReadCsvFromDataSlice(rows, s, option)
}

// key-value格式的csv数据给对象赋值
// V支持proto.Message和普通struct结构
func ReadCsvFileObject[V any](file string, v V, option *CsvOption) error {
	rows, readErr := ReadCsvFile(file)
	if readErr != nil {
		return readErr
	}
	return ReadCsvFromDataObject(rows, v, option)
}

func ReadCsvFile(file string) ([][]string, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return csv.NewReader(f).ReadAll()
}

// csv数据转换成map
// V支持proto.Message和普通struct结构
func ReadCsvFromDataMap[M ~map[K]V, K IntOrString, V any](rows [][]string, m M, option *CsvOption) error {
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
	valueType := mType.Elem() // value type of m, 如*pb.ItemCfg or pb.ItemCfg
	for rowIndex := option.DataBeginRowIndex; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex]
		// 固定第一列是key
		key := gentity.ConvertStringToRealType(keyType, row[0])
		value := ConvertCsvLineToValue(valueType, row, columnNames, option)
		mVal.SetMapIndex(reflect.ValueOf(key), value)
	}
	return nil
}

// csv数据转换成slice
// V支持proto.Message和普通struct结构
func ReadCsvFromDataSlice[Slice ~[]V, V any](rows [][]string, s Slice, option *CsvOption) (Slice, error) {
	if len(rows) == 0 {
		return s, errors.New("no csv header")
	}
	columnNames := rows[0]
	if len(columnNames) == 0 {
		return s, errors.New("no column")
	}
	if option.DataBeginRowIndex < 1 {
		return s, errors.New("DataBeginRowIndex must >=1")
	}
	sType := reflect.TypeOf(s)
	valueType := sType.Elem() // value type of s, 如*pb.ItemCfg or pb.ItemCfg
	for rowIndex := option.DataBeginRowIndex; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex]
		value := ConvertCsvLineToValue(valueType, row, columnNames, option)
		s = slices.Insert(s, len(s), value.Interface().(V)) // s = append(s, value)
	}
	return s, nil
}

// key-value格式的csv数据转换成对象
// V支持proto.Message和普通struct结构
func ReadCsvFromDataObject[V any](rows [][]string, v V, option *CsvOption) error {
	if len(rows) == 0 {
		return errors.New("no csv header")
	}
	if len(rows[0]) < 2 {
		return errors.New("column count must >= 2")
	}
	if option.DataBeginRowIndex < 1 {
		return errors.New("DataBeginRowIndex must >=1")
	}
	typ := reflect.TypeOf(v) // type of v, 如*pb.ItemCfg or pb.ItemCfg
	val := reflect.ValueOf(v)
	if typ.Kind() != reflect.Ptr {
		return errors.New("v must be Ptr")
	}
	valElem := val.Elem() // *pb.ItemCfg -> pb.ItemCfg
	for rowIndex := option.DataBeginRowIndex; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex]
		// key-value的固定格式,列名不用
		columnName := row[0]
		fieldString := row[1]
		fieldVal := valElem.FieldByName(columnName)
		if fieldVal.Kind() == reflect.Ptr { // 指针类型的字段,如 Name *string
			fieldObj := reflect.New(fieldVal.Type().Elem()) // 如new(string)
			fieldVal.Set(fieldObj)                          // 如 obj.Name = new(string)
			fieldVal = fieldObj.Elem()                      // 如 *(obj.Name)
		}
		ConvertStringToFieldValue(val, fieldVal, columnName, fieldString, option)
	}
	return nil
}

// 字段赋值,根据字段的类型,把字符串转换成对应的值
func ConvertStringToFieldValue(object, fieldVal reflect.Value, columnName, fieldString string, option *CsvOption) {
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
		// 自定义的转换接口
		v := fieldConverter(object.Interface(), columnName, fieldString)
		fieldVal.Set(reflect.ValueOf(v))
	} else {
		var convertToElem bool
		fieldConverter, convertToElem = option.GetConverterByTypePtrOrStruct(fieldVal.Type())
		if fieldConverter != nil {
			// 自定义的转换接口
			v := fieldConverter(object.Interface(), columnName, fieldString)
			if v == nil {
				slog.Debug("field parse error", "columnName", columnName, "fieldString", fieldString)
				return
			}
			if convertToElem {
				fieldVal.Set(reflect.ValueOf(v).Elem())
			} else {
				fieldVal.Set(reflect.ValueOf(v))
			}
			return
		}
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
				slog.Error("float64 convert error:", "columnName", columnName, "fieldString", fieldString, "err", err)
				break
			}
			fieldVal.SetFloat(f32)

		case reflect.Float64:
			f64, err := strconv.ParseFloat(fieldString, 64)
			if err != nil {
				slog.Error("float64 convert error:", "columnName", columnName, "fieldString", fieldString, "err", err)
				break
			}
			fieldVal.SetFloat(f64)

		case reflect.Bool:
			fieldVal.SetBool(fieldString == "true" || fieldString == "1")

		case reflect.Slice:
			// 常规数组解析
			if fieldString == "" {
				return
			}
			newSlice := reflect.MakeSlice(fieldVal.Type(), 0, 0)
			sliceElemType := fieldVal.Type().Elem()
			converter, convertToElem := option.GetConverterByTypePtrOrStruct(sliceElemType)
			strs := strings.Split(fieldString, option.SliceSeparator)
			for _, str := range strs {
				if str == "" {
					continue
				}
				var sliceElemValue interface{}
				if converter != nil {
					sliceElemValue = converter(object.Interface(), columnName, str)
				} else {
					sliceElemValue = gentity.ConvertStringToRealType(sliceElemType, str)
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
				kvStr := strings.Split(mapItemStr, option.MapKVSeparator)
				if len(kvStr) == 2 {
					fieldKeyValue := gentity.ConvertStringToRealType(fieldKeyType, kvStr[0])
					var fieldValueValue interface{}
					if converter != nil {
						fieldValueValue = converter(object.Interface(), columnName, kvStr[1])
					} else {
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
		ConvertStringToFieldValue(newObject, fieldVal, columnName, fieldString, option)
	}
	return newObject
}
