package tool

import (
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func init() {
	debugLevel := &slog.LevelVar{}
	debugLevel.Set(slog.LevelDebug)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: debugLevel,
	})))
	slog.Debug("test init")
}

func TestReadCsvFromData(t *testing.T) {
	rows := [][]string{
		{"CfgId", "Name", "Detail", "Unique", "unknownColumn", "SliceTest", "MapTest"},
		{"1", "普通物品1", "普通物品1详细信息", "false", "123", "1_2_3", "a_1#b_2#c_3"},
		{"2", "普通物品2", "普通物品2详细信息", "false", "test", "4", "d_4"},
		{"3", "装备3", "装备3详细信息", "true", "", "", ""},
	}
	option := NewCsvOption(1)
	// proto.Message格式的map
	m := make(map[int32]*pb.ItemCfg)
	err := ReadCsvFromData(rows, m, option)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range m {
		t.Logf("%v", item)
	}

	// 测试非proto.Message的map格式
	type testItemCfg struct {
		CfgId     int32
		Name      string
		Detail    string
		Unique    bool
		SliceTest []int
		MapTest   map[string]int32
	}
	// slice分隔符
	option.SliceSeparator = "_"
	option.MapSeparator = "#"
	option.MapKVSeparator = "_"
	m2 := make(map[int]*testItemCfg)
	err = ReadCsvFromData(rows, m2, option)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range m2 {
		t.Logf("%v", item)
	}
}

func TestReadCsvFromDataCustomConvert(t *testing.T) {
	rows := [][]string{
		{"CfgId", "Name", "Item"},
		{"1", "Name1", "123_1"},
		{"2", "Name2", "456_5"},
		{"3", "Name3", "789_10"},
	}
	option := NewCsvOption(1).
		AddFieldConverter("Item", func(obj interface{}, columnName, fieldStr string) interface{} {
			strs := strings.Split(fieldStr, "_")
			return &pb.ItemNum{
				CfgId: int32(util.Atoi(strs[0])),
				Num:   int32(util.Atoi(strs[1])),
			}
		})
	type testCfg struct {
		CfgId int32
		Name  string
		Item  *pb.ItemNum
	}
	m := make(map[int]*testCfg)
	err := ReadCsvFromData(rows, m, option)
	if err != nil {
		t.Fatal(err)
	}
	for _, cfg := range m {
		t.Logf("%v", cfg)
	}
}
