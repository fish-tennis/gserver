package tool

import (
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"testing"
)

func init() {
	debugLevel := &slog.LevelVar{}
	debugLevel.Set(slog.LevelDebug)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
		Level:     debugLevel,
	})))
	slog.Debug("test init")
}

func TestReadCsvFromDataProto(t *testing.T) {
	rows := [][]string{
		{"CfgId", "Name", "Detail", "Unique", "unknownColumnTest"},
		{"1", "普通物品1", "普通物品1详细信息", "false", "123"},
		{"2", "普通物品2", "普通物品2详细信息", "false", "test"},
		{"3", "装备3", "装备3详细信息", "true", ""},
	}
	option := &CsvOption{
		DataBeginRowIndex: 1,
		SliceSeparator:    ";",
		MapKVSeparator:    "_",
		MapSeparator:      ";",
	}
	// proto.Message格式的map
	m := make(map[int32]*pb.ItemCfg)
	err := ReadCsvFromData(rows, m, option)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range m {
		t.Logf("%v", item)
	}
}

func TestReadCsvFromDataStruct(t *testing.T) {
	rows := [][]string{
		{"Name", "Detail", "Unique", "SliceTest", "MapTest"},
		{"item1", "普通物品1详细信息", "false", "1;2;3", "a_1;b_2;c_3"},
		{"item2", "普通物品2详细信息", "false", "4", "d_4"},
		{"item3", "装备3详细信息", "true", "", ""},
	}
	option := &CsvOption{
		DataBeginRowIndex: 1,
		SliceSeparator:    ";",
		MapKVSeparator:    "_",
		MapSeparator:      ";",
	}
	// 测试非proto.Message的map格式
	type testItemCfg struct {
		Name      string
		Detail    string
		Unique    bool
		SliceTest []int
		MapTest   map[string]int32
	}
	// map的key也可以是字符串
	m := make(map[string]*testItemCfg)
	err := ReadCsvFromData(rows, m, option)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range m {
		t.Logf("%v", item)
	}
}

func TestReadCsvFromDataConverter(t *testing.T) {
	rows := [][]string{
		{"CfgId", "Name", "Item", "Items", "ColorFlags", "Color"},
		{"1", "Name1", "123_1", "123_1;456_2", "Red;Green;Blue", "Red"},
		{"2", "Name2", "456_5", "123_1", "Gray;Yellow", "Gray"},
		{"3", "Name3", "789_10", "", "", ""},
	}
	type testCfg struct {
		CfgId      int32
		Name       string
		Item       *pb.ItemNum
		Items      []*pb.ItemNum
		ColorFlags int32 // 颜色的组合值,如 Red | Green
		Color      pb.Color
	}

	option := &CsvOption{
		DataBeginRowIndex: 1,
		SliceSeparator:    ";",
		MapKVSeparator:    "_",
		MapSeparator:      ";",
	}
	// 注册列名对应的解析接口
	// 这里的ColorFlags列演示了一种特殊需求: 颜色的组合值用更易读的方式在csv中填写
	option.RegisterConverterByColumnName("ColorFlags", func(obj interface{}, columnName, fieldStr string) interface{} {
		t.Logf("ColorFlags parse columnName:%s,fieldStr:%s", columnName, fieldStr)
		colorStrs := strings.Split(fieldStr, ";")
		flags := int32(0)
		for _, colorStr := range colorStrs {
			if colorValue, ok := pb.Color_value["Color_"+colorStr]; ok {
				flags |= colorValue
			}
		}
		return flags
	})
	// 注册pb.ItemNum的解析接口
	option.RegisterConverterByType(reflect.TypeOf(&pb.ItemNum{}), func(obj interface{}, columnName, fieldStr string) interface{} {
		strs := strings.Split(fieldStr, "_")
		if len(strs) != 2 {
			return nil
		}
		return &pb.ItemNum{
			CfgId: int32(util.Atoi(strs[0])),
			Num:   int32(util.Atoi(strs[1])),
		}
	})
	// 注册颜色枚举的自定义解析接口,csv中可以直接填写颜色对应的字符串
	option.RegisterConverterByType(reflect.TypeOf(pb.Color(0)), func(obj interface{}, columnName, fieldStr string) interface{} {
		t.Logf("pb.Color parse columnName:%s,fieldStr:%s", columnName, fieldStr)
		if colorValue, ok := pb.Color_value["Color_"+fieldStr]; ok {
			return pb.Color(colorValue)
		}
		return pb.Color(0)
	})

	m := make(map[int]*testCfg)
	err := ReadCsvFromData(rows, m, option)
	if err != nil {
		t.Fatal(err)
	}
	for _, cfg := range m {
		t.Logf("%v", cfg)
	}
}
