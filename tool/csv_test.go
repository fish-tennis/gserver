package tool

import (
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var (
	defaultOption = CsvOption{
		DataBeginRowIndex: 1,
		SliceSeparator:    ";",
		MapKVSeparator:    "_",
		MapSeparator:      "#",
	}
)

func init() {
	debugLevel := &slog.LevelVar{}
	debugLevel.Set(slog.LevelDebug)
	slog.SetDefault(slog.New(logger.NewJsonHandlerWithStdOutput(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     debugLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				source := a.Value.Any().(*slog.Source)
				// 让source简短些
				if wd, err := os.Getwd(); err == nil {
					source.File = strings.TrimPrefix(source.File, filepath.ToSlash(wd))
				}
			}
			return a
		},
	}, true)))
	slog.Debug("test init")
}

func TestReadCsvFromDataProto(t *testing.T) {
	rows := [][]string{
		{"CfgId", "Name", "Detail", "Unique", "unknownColumnTest"},
		{"1", "普通物品1", "普通物品1详细信息", "false", "123"},
		{"2", "普通物品2", "普通物品2详细信息", "false", "test"},
		{"3", "装备3", "装备3详细信息", "true", ""},
	}
	// proto.Message格式的map
	m := make(map[int32]*pb.ItemCfg)
	err := ReadCsvFromDataMap(rows, m, &defaultOption)
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
		{"item1", "普通物品1详细信息", "false", "1;2;3", "a_1#b_2#c_3"},
		{"item2", "普通物品2详细信息", "false", "4", "d_4"},
		{"item3", "装备3详细信息", "true", "", ""},
	}
	// 测试非proto.Message的map格式
	type testItemCfg struct {
		Name      string
		Detail    *string // 测试指针类型的字段
		Unique    bool
		SliceTest []int
		MapTest   map[string]int32
	}
	// map的key也可以是字符串
	m := make(map[string]testItemCfg)
	err := ReadCsvFromDataMap(rows, m, &defaultOption)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range m {
		t.Logf("%v", item)
		t.Logf("Detail:%v", *item.Detail)
	}
}

func TestReadCsvFromDataConverter(t *testing.T) {
	rows := [][]string{
		{"CfgId", "Name", "Item", "Items", "ColorFlags", "Color", "ColorPtr", "ItemStruct", "ItemStructs", "ItemMap"},
		{"1", "Name1", "123_1", "123_1;456_2", "Red;Green;Blue", "Red", "Red", "123_1", "321_1;654_2", "1_321_1#2_654_2"},
		{"2", "Name2", "456_5", "123_1", "Gray;Yellow", "Gray", "Gray", "456_5", "321_1", "1_321_1"},
		{"3", "Name3", "789_10", "", "", "", "", "", "", ""},
	}
	type testCfg struct {
		CfgId      int32
		Name       string
		Item       *pb.ItemNum
		ItemStruct pb.ItemNum // 注册了&pb.ItemNum{}接口,pb.ItemNum也会被正确解析

		Items       []*pb.ItemNum
		ItemStructs []pb.ItemNum // 注册了&pb.ItemNum{}接口,pb.ItemNum也会被正确解析

		ItemMap map[int32]*pb.ItemNum

		Color      pb.Color
		ColorPtr   *pb.Color // 注册了pb.Color接口,*pb.Color也会被正确解析
		ColorFlags int32     // 颜色的组合值,如 Red | Green
	}

	option := defaultOption

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
		t.Logf("pb.Color parse columnName:%v,fieldStr:%v", columnName, fieldStr)
		if colorValue, ok := pb.Color_value["Color_"+fieldStr]; ok {
			return pb.Color(colorValue)
		}
		return pb.Color(0)
	})
	// 注册列名对应的解析接口
	// 这里的ColorFlags列演示了一种特殊需求: 颜色的组合值用更易读的方式在csv中填写
	option.RegisterConverterByColumnName("ColorFlags", func(obj interface{}, columnName, fieldStr string) interface{} {
		colorStrs := strings.Split(fieldStr, ";")
		flags := int32(0)
		for _, colorStr := range colorStrs {
			if colorValue, ok := pb.Color_value["Color_"+colorStr]; ok && colorValue > 0 {
				flags |= 1 << (colorValue - 1)
			}
		}
		t.Logf("ColorFlags parse columnName:%v,fieldStr:%v flags:%v", columnName, fieldStr, flags)
		return flags
	})

	m := make(map[int]*testCfg)
	err := ReadCsvFromDataMap(rows, m, &option)
	if err != nil {
		t.Fatal(err)
	}
	for _, cfg := range m {
		t.Logf("%v", cfg)
		t.Logf("%v", cfg.Item)
		t.Logf("%v", &cfg.ItemStruct)
		t.Logf("%v", cfg.Items)
		t.Logf("%v", cfg.ItemStructs)
		t.Logf("%v", cfg.ItemMap)
	}
}

func TestMapReflect(t *testing.T) {
	type testStruct struct {
		I int
		S string
	}
	m := make(map[int]testStruct)
	mType := reflect.TypeOf(m)
	mVal := reflect.ValueOf(m)
	//keyType := mType.Key()    // int
	valueType := mType.Elem() // pb.ItemCfg
	t.Logf("%v", valueType)
	key := 1
	newItem := reflect.New(valueType) // new(pb.ItemCfg)
	newItem.Elem().FieldByName("I").SetInt(123)
	newItem.Elem().FieldByName("S").SetString("abc")
	mVal.SetMapIndex(reflect.ValueOf(key), newItem.Elem())
	for _, cfg := range m {
		t.Logf("%v", cfg)
	}
}

func TestReadCsvFromDataSlice(t *testing.T) {
	rows := [][]string{
		{"CfgId", "Name", "Detail", "Unique", "unknownColumnTest"},
		{"1", "普通物品1", "普通物品1详细信息", "false", "123"},
		{"2", "普通物品2", "普通物品2详细信息", "false", "test"},
		{"3", "装备3", "装备3详细信息", "true", ""},
	}
	s := make([]*pb.ItemCfg, 0)
	newSlice, err := ReadCsvFromDataSlice(rows, s, &defaultOption)
	if err != nil {
		t.Fatal(err)
	}
	for i, item := range newSlice {
		t.Logf("%v: %v", i, item)
	}
}

func TestReadCsvFromDataObject(t *testing.T) {
	rows := [][]string{
		{"Key", "Value", "unknownColumnTest"},
		{"CfgId", "123", "comment1"},
		{"Name", "物品名", "comment2"},
		{"Detail", "物品详情", "comment3"},
		{"Unique", "true", "comment4"},
	}
	type testCfg struct {
		CfgId  int32
		Name   string
		Detail *string
		Unique bool
	}
	obj := new(testCfg)
	err := ReadCsvFromDataObject(rows, obj, &defaultOption)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%v", obj)
	t.Logf("Detail:%v", *obj.Detail)
}
