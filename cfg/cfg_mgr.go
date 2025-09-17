package cfg

import (
	"encoding/json"
	"github.com/fish-tennis/gserver/internal"
	"log/slog"
	"os"
	"slices"
)

// map类型的配置数据管理
type DataMap[E internal.CfgData] struct {
	Elems map[int32]E
}

func NewDataMap[E internal.CfgData]() *DataMap[E] {
	return &DataMap[E]{
		Elems: make(map[int32]E),
	}
}

func (this *DataMap[E]) GetCfg(cfgId int32) E {
	return this.Elems[cfgId]
}

func (this *DataMap[E]) Range(f func(e E) bool) {
	for _, cfg := range this.Elems {
		if !f(cfg) {
			return
		}
	}
}

// 加载配置数据
func (this *DataMap[E]) Load(fileName string) error {
	if this.Elems == nil {
		this.Elems = make(map[int32]E)
	}
	return this.LoadJson(fileName)
}

// 从json文件加载数据
func (this *DataMap[E]) LoadJson(fileName string) error {
	fileData, err := os.ReadFile(fileName)
	if err != nil {
		slog.Error("LoadJsonErr", "fileName", fileName, "err", err)
		return err
	}
	cfgMap := make(map[int32]E)
	err = json.Unmarshal(fileData, &cfgMap)
	if err != nil {
		slog.Error("LoadJsonErr", "fileName", fileName, "err", err)
		return err
	}
	this.Elems = cfgMap
	slog.Info("LoadJson", "fileName", fileName, "count", len(this.Elems))
	return nil
}

// 创建索引
func (this *DataMap[E]) CreateIndexInt32(indexFn func(e E) int32) map[int32]*DataMap[E] {
	indexMap := make(map[int32]*DataMap[E])
	for _, e := range this.Elems {
		index := indexFn(e)
		if indexMap[index] == nil {
			indexMap[index] = NewDataMap[E]()
		}
		indexMap[index].Elems[e.GetCfgId()] = e
	}
	return indexMap
}

// 创建子集
func (this *DataMap[E]) CreateSubset(filter func(e E) bool) *DataMap[E] {
	subMap := NewDataMap[E]()
	for _, e := range this.Elems {
		if filter(e) {
			subMap.Elems[e.GetCfgId()] = e
		}
	}
	return subMap
}

// 创建slice
func (this *DataMap[E]) CreateSlice(filter func(e E) bool, cmpFn func(a, b E) int) []E {
	var s []E
	for _, e := range this.Elems {
		if filter(e) {
			s = append(s, e)
		}
	}
	if cmpFn != nil {
		slices.SortFunc(s, cmpFn)
	}
	return s
}

// slice类型的配置数据管理
type DataSlice[E any] struct {
	Elems []E
}

func (this *DataSlice[E]) Len() int {
	return len(this.Elems)
}

func (this *DataSlice[E]) GetCfg(index int) E {
	return this.Elems[index]
}

func (this *DataSlice[E]) Range(f func(e E) bool) {
	for _, cfg := range this.Elems {
		if !f(cfg) {
			return
		}
	}
}

// 加载配置数据
func (this *DataSlice[E]) Load(fileName string) error {
	return this.LoadJson(fileName)
}

// 从json文件加载数据
func (this *DataSlice[E]) LoadJson(fileName string) error {
	fileData, err := os.ReadFile(fileName)
	if err != nil {
		slog.Error("LoadJsonErr", "fileName", fileName, "err", err)
		return err
	}
	var cfgList []E
	err = json.Unmarshal(fileData, &cfgList)
	if err != nil {
		slog.Error("LoadJsonErr", "fileName", fileName, "err", err)
		return err
	}
	this.Elems = cfgList
	slog.Info("LoadJson", "fileName", fileName, "count", len(this.Elems))
	this.checkDuplicateCfgId(fileName)
	return nil
}

// 如果配置项是CfgData,检查id是否重复
func (this *DataSlice[E]) checkDuplicateCfgId(fileName string) {
	for i := 0; i < len(this.Elems); i++ {
		cfgDataI, ok := any(this.Elems[i]).(internal.CfgData)
		if !ok {
			return
		}
		for j := i + 1; j < len(this.Elems); j++ {
			cfgDataJ := any(this.Elems[j]).(internal.CfgData)
			if cfgDataI.GetCfgId() == cfgDataJ.GetCfgId() {
				slog.Error("duplicate id", "fileName", fileName, "id", cfgDataI.GetCfgId())
			}
		}
	}
}
