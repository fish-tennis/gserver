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

	// 配置支持的数值比较符
	_valueOps = []string{
		"=", ">", ">=", "<", "<=", "!=", "[]", "![]",
	}
)

func init() {
	// ValueCompareCfg较复杂,且大部分情况都只使用默认的操作符=,所以特殊处理ValueCompareCfg的解析,降低配置难度
	// 如: IsWin_1#RoomType_2;3#RoomLevel_>3#Score_[]100;200
	DefaultCsvOption.RegisterConverterByType(reflect.TypeOf(&pb.ValueCompareCfg{}), func(obj any, columnName, fieldStr string) any {
		return convertValueCompareCfg(fieldStr)
	})
}

func convertValueCompareCfg(fieldStr string) *pb.ValueCompareCfg {
	v := &pb.ValueCompareCfg{
		Op: "=", // 简化配置,不填就默认是=
	}
	valueIdx := 0
	for _, op := range _valueOps {
		idx := strings.Index(fieldStr, op)
		if idx == 0 {
			v.Op = op
			valueIdx = len(op)
			break
		}
	}
	valueStr := fieldStr[valueIdx:]
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
