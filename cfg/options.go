package cfg

import (
	"github.com/fish-tennis/csv"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"reflect"
	"strings"
)

// 默认csv设置
var (
	DefaultCsvOption = csv.DefaultOption
)

func init() {
	DefaultCsvOption.IgnoreColumn("Comment")

	// ValueCompareCfg较复杂,且大部分情况都只使用默认的操作符=,所以特殊处理ValueCompareCfg的解析,降低配置难度
	// 如: IsWin_1#RoomType_2;3#RoomLevel_>_3#Score_[]_100;200
	DefaultCsvOption.RegisterConverterByType(reflect.TypeOf(&pb.ValueCompareCfg{}), func(obj any, columnName, fieldStr string) any {
		return convertValueCompareCfg(fieldStr)
	})
}

// ValueCompareCfg较复杂,且大部分情况都只使用默认的操作符=,所以特殊处理ValueCompareCfg的解析,降低配置难度
// 如: IsWin_1#RoomType_2;3#RoomLevel_>_3#Score_[]_100;200
func convertValueCompareCfg(fieldStr string) *pb.ValueCompareCfg {
	opValues := strings.Split(fieldStr, DefaultCsvOption.KvSeparator)
	if len(opValues) == 0 {
		return nil
	}
	v := &pb.ValueCompareCfg{}
	var valueStr string
	if len(opValues) == 1 {
		v.Op = "=" // len(opValues)==0表示使用默认的比较操作符=,可以不配置,如RoomType_2
		valueStr = opValues[0]
	} else {
		// op_values,如RoomLevel_>_3
		v.Op = opValues[0]
		valueStr = opValues[1]
	}
	values := strings.Split(valueStr, DefaultCsvOption.SliceSeparator)
	for _, value := range values {
		valueInt := util.ToInt(value)
		if valueInt == 0 {
			// true和false转换成1和0
			valueLower := strings.ToLower(value)
			if valueLower == "true" {
				valueInt = 1
			} else if valueLower == "false" {
				valueInt = 0
			}
		}
		v.Values = append(v.Values, int32(valueInt))
	}
	return v
}
