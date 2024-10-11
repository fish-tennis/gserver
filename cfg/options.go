package cfg

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/tool"
	"log/slog"
	"reflect"
	"strings"
)

// 默认csv设置
var DefaultCsvOption = tool.CsvOption{
	ColumnNameRowIndex: 0,
	DataBeginRowIndex:  1, // csv行索引
	SliceSeparator:     ";",
	MapKVSeparator:     "_",
	MapSeparator:       "#",
}

func init() {
	// ProgressCfg有map[string]any动态属性,需要注册自定义解析接口
	DefaultCsvOption.RegisterConverterByType(reflect.TypeOf(&internal.ProgressCfg{}), func(obj any, columnName, fieldStr string) any {
		// Type_2#Total_5#Properties_{IsPvp_true#IsWin_true}#Key_k
		cfg := &internal.ProgressCfg{}
		cfg.Properties = make(map[string]any)
		parseCsvWithProperties(cfg, fieldStr)
		return cfg
	})
}

// 解析类似ProgressCfg那种带map[string]any或map[string]string的结构
// 如Type_2#Total_5#Properties_{IsPvp_true#IsWin_true}#Key_k
func parseCsvWithProperties(cfg any, fieldStr string) {
	objVal := reflect.ValueOf(cfg).Elem()
	var cutStrings []string
	propertiesString := ""
	propertiesKey := "Properties_{"
	propertiedBeginPos := strings.Index(fieldStr, propertiesKey)
	if propertiedBeginPos >= 0 {
		propertiedEndPos := strings.Index(fieldStr, "}")
		if propertiedEndPos > propertiedBeginPos {
			propertiesString = fieldStr[propertiedBeginPos+len(propertiesKey) : propertiedEndPos]
			if propertiedBeginPos > 0 {
				cutStrings = append(cutStrings, fieldStr[:propertiedBeginPos-1]) // 不包含Properties_前面的#
			} else {
				cutStrings = append(cutStrings, fieldStr[:propertiedBeginPos])
			}
			if propertiedEndPos < len(fieldStr)-1 {
				cutStrings = append(cutStrings, fieldStr[propertiedEndPos+1:])
			}
		}
	}
	if len(cutStrings) == 0 {
		cutStrings = append(cutStrings, fieldStr)
	}
	for _, cutString := range cutStrings {
		// Type_2#Total_5
		fieldSlice := strings.Split(cutString, "#")
		for _, kvPairString := range fieldSlice {
			// Type_2
			kvPair := strings.Split(kvPairString, "_")
			if len(kvPair) != 2 {
				slog.Error("kvPairError", "kvPairString", kvPairString)
				continue
			}
			fieldVal := objVal.FieldByName(kvPair[0])
			tool.ConvertStringToFieldValue(objVal, fieldVal, kvPair[0], kvPair[1], &DefaultCsvOption, true)
		}
	}
	// IsPvp_true#IsWin_true
	if len(propertiesString) > 0 {
		propertiesVal := objVal.FieldByName("Properties")
		fieldSlice := strings.Split(propertiesString, "#")
		for _, kvPairString := range fieldSlice {
			// IsPvp_true
			kvPair := strings.Split(kvPairString, "_")
			if len(kvPair) != 2 {
				slog.Error("kvPairError", "kvPairString", kvPairString)
				continue
			}
			// csv格式,无法区分值类型,统一用string
			propertiesVal.SetMapIndex(reflect.ValueOf(kvPair[0]), reflect.ValueOf(kvPair[1]))
		}
	}
}
