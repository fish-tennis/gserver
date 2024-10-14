package cfg

import (
	"github.com/fish-tennis/csv"
	"github.com/fish-tennis/gserver/internal"
	"log/slog"
	"reflect"
	"strings"
)

// 默认csv设置
var DefaultCsvOption = csv.DefaultOption

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
	pairs := csv.ParseNestString(fieldStr, &DefaultCsvOption, "Properties")
	for _, pair := range pairs {
		fieldVal := objVal.FieldByName(pair.Key)
		if pair.Key != "Properties" {
			csv.ConvertStringToFieldValue(objVal, fieldVal, pair.Key, pair.Value, &DefaultCsvOption, true)
		} else {
			pairSlice := strings.Split(pair.Value, DefaultCsvOption.PairSeparator)
			for _, kvPairString := range pairSlice {
				// IsPvp_true
				kvPair := strings.SplitN(kvPairString, DefaultCsvOption.KvSeparator, 2)
				if len(kvPair) != 2 {
					slog.Error("kvPairError", "kvPairString", kvPairString)
					continue
				}
				// csv格式,无法区分值类型,统一用string
				fieldVal.SetMapIndex(reflect.ValueOf(kvPair[0]), reflect.ValueOf(kvPair[1]))
			}
		}
	}
}
