package cfg

import (
	"github.com/fish-tennis/csv"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
	"reflect"
)

// 默认csv设置
var DefaultCsvOption = csv.DefaultOption

func init() {
	// ProgressCfg有map[string]any动态属性,需要注册自定义解析接口
	DefaultCsvOption.RegisterConverterByType(reflect.TypeOf(&internal.ProgressCfg{}), parseProgressCfg)

	// ActivityCfg.Exchanges []*ExchangeCfg是个嵌套子结构的数组,需要注册自定义解析接口
	var exchangeCfgSlice []*pb.ExchangeCfg
	DefaultCsvOption.RegisterConverterByType(reflect.TypeOf(exchangeCfgSlice), parseExchangeCfgSlice)

	// any类型需要注册自定义解析接口
	var mapStringAny map[string]any
	DefaultCsvOption.RegisterConverterByType(reflect.TypeOf(mapStringAny), parseMapStringAny)
}

func parseProgressCfg(obj any, columnName, fieldStr string) any {
	// Type_2#Total_5#Properties_{IsPvp_true#IsWin_true}#Key_k
	cfg := &internal.ProgressCfg{}
	cfg.Properties = make(map[string]any)
	parseCsvWithProperties(cfg, fieldStr)
	return cfg
}

// 解析类似ProgressCfg那种带map[string]any或map[string]string的结构
// 如Type_2#Total_5#Properties_{IsPvp_true#IsWin_true}#Key_k
func parseCsvWithProperties(cfg any, fieldStr string) {
	objVal := reflect.ValueOf(cfg).Elem()
	pairs := csv.ParseNestString(fieldStr, &DefaultCsvOption, "Properties")
	for _, pair := range pairs {
		fieldVal := objVal.FieldByName(pair.Key)
		if pair.Key != "Properties" {
			// 普通字段
			csv.ConvertStringToFieldValue(objVal, fieldVal, pair.Key, pair.Value, &DefaultCsvOption, true)
		} else {
			// map[string]any特殊处理
			propertiesPairs := csv.ParsePairString(pair.Value, &DefaultCsvOption)
			for _, propertiesPair := range propertiesPairs {
				// csv格式,无法区分值类型,统一用string
				fieldVal.SetMapIndex(reflect.ValueOf(propertiesPair.Key), reflect.ValueOf(propertiesPair.Value))
			}
		}
	}
}

// []*pb.ExchangeCfg
func parseExchangeCfgSlice(obj any, columnName, fieldStr string) any {
	// CfgId_1#ConsumeItems_{CfgId_1#Num_1;CfgId_2#Num_2}#Rewards_{CfgId_3#Num_1};
	// CfgId_2#ConsumeItems_{CfgId_3#Num_1;CfgId_4#Num_2}#Rewards_{CfgId_1#Num_10}
	var exchanges []*pb.ExchangeCfg
	// pairsSlice:
	// [
	//  [{K:CfgId,V:1},{K:ConsumeItems,V:CfgId_1#Num_1;CfgId_2#Num_2},{K:Rewards,V:CfgId_3#Num_1}],
	//  [{K:CfgId,V:2},{K:ConsumeItems,V:CfgId_3#Num_1;CfgId_4#Num_2},{K:Rewards,V:CfgId_1#Num_10}]
	// ]
	pairsSlice := csv.ParseNestStringSlice(fieldStr, &DefaultCsvOption, "ConsumeItems", "Rewards")
	for _, pairs := range pairsSlice {
		slog.Info("pairsSlice", "pairs", pairs)
		exchangeCfg := &pb.ExchangeCfg{}
		objVal := reflect.ValueOf(exchangeCfg).Elem()
		// pairs:
		// [{K:CfgId,V:1},{K:ConsumeItems,V:CfgId_1#Num_1;CfgId_2#Num_2},{K:Rewards,V:CfgId_3#Num_1}]
		for _, pair := range pairs {
			slog.Info("pair of ExchangeCfg", "k", pair.Key, "v", pair.Value)
			fieldVal := objVal.FieldByName(pair.Key)
			csv.ConvertStringToFieldValue(objVal, fieldVal, pair.Key, pair.Value, &DefaultCsvOption, false)
		}
		exchanges = append(exchanges, exchangeCfg)
	}
	return exchanges
}

// map[string]any
func parseMapStringAny(obj any, columnName, fieldStr string) any {
	m := make(map[string]any)
	pairs := csv.ParsePairString(fieldStr, &DefaultCsvOption)
	for _, pair := range pairs {
		m[pair.Key] = pair.Value
	}
	return m
}
