package cfg

import (
	"encoding/json"
	"errors"
	"github.com/fish-tennis/csv"
	"github.com/fish-tennis/gserver/internal"
	"log/slog"
	"os"
	"strings"
)

// map类型的配置数据管理
type DataMap[E internal.CfgData] struct {
	cfgs map[int32]E
}

func (this *DataMap[E]) GetCfg(cfgId int32) E {
	return this.cfgs[cfgId]
}

func (this *DataMap[E]) Range(f func(e E) bool) {
	for _, cfg := range this.cfgs {
		if !f(cfg) {
			return
		}
	}
}

// 加载配置数据,支持json和csv
func (this *DataMap[E]) Load(fileName string, loaderOption *LoaderOption) error {
	if this.cfgs == nil {
		this.cfgs = make(map[int32]E)
	}
	if strings.HasSuffix(fileName, ".json") {
		return this.LoadJson(fileName)
	} else if strings.HasSuffix(fileName, ".csv") {
		return this.LoadCsv(fileName, loaderOption.CsvOption)
	}
	return errors.New("unsupported file type")
}

// 从json文件加载数据
func (this *DataMap[E]) LoadJson(fileName string) error {
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
	cfgMap := make(map[int32]E, len(cfgList))
	for _, cfg := range cfgList {
		if _, exists := cfgMap[cfg.GetCfgId()]; exists {
			slog.Error("duplicate id", "fileName", fileName, "id", cfg.GetCfgId())
		}
		cfgMap[cfg.GetCfgId()] = cfg
	}
	this.cfgs = cfgMap
	slog.Info("LoadJson", "fileName", fileName, "count", len(this.cfgs))
	return nil
}

// 从csv文件加载数据
func (this *DataMap[E]) LoadCsv(fileName string, option *csv.CsvOption) error {
	cfgMap := make(map[int32]E)
	err := csv.ReadCsvFileMap(fileName, cfgMap, option)
	if err != nil {
		slog.Error("LoadCsvErr", "fileName", fileName, "err", err)
		return err
	}
	this.cfgs = cfgMap
	slog.Info("LoadCsv", "fileName", fileName, "count", len(this.cfgs))
	return nil
}

// slice类型的配置数据管理
type DataSlice[E any] struct {
	cfgs []E
}

func (this *DataSlice[E]) Len() int {
	return len(this.cfgs)
}

func (this *DataSlice[E]) GetCfg(index int) E {
	return this.cfgs[index]
}

func (this *DataSlice[E]) Range(f func(e E) bool) {
	for _, cfg := range this.cfgs {
		if !f(cfg) {
			return
		}
	}
}

// 加载配置数据,支持json和csv
func (this *DataSlice[E]) Load(fileName string, loaderOption *LoaderOption) error {
	if strings.HasSuffix(fileName, ".json") {
		return this.LoadJson(fileName)
	} else if strings.HasSuffix(fileName, ".csv") {
		return this.LoadCsv(fileName, loaderOption.CsvOption)
	}
	return errors.New("unsupported file type")
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
	this.cfgs = cfgList
	slog.Info("LoadJson", "fileName", fileName, "count", len(this.cfgs))
	this.checkDuplicateCfgId(fileName)
	return nil
}

func (this *DataSlice[E]) LoadCsv(fileName string, option *csv.CsvOption) error {
	s := make([]E, 0)
	var err error
	this.cfgs, err = csv.ReadCsvFileSlice(fileName, s, option)
	if err != nil {
		slog.Error("LoadCsvErr", "fileName", fileName, "err", err)
		return err
	}
	slog.Info("LoadCsv", "fileName", fileName, "count", len(this.cfgs))
	this.checkDuplicateCfgId(fileName)
	return nil
}

// 如果配置项是CfgData,检查id是否重复
func (this *DataSlice[E]) checkDuplicateCfgId(fileName string) {
	for i := 0; i < len(this.cfgs); i++ {
		cfgDataI, ok := any(this.cfgs[i]).(internal.CfgData)
		if !ok {
			return
		}
		for j := i + 1; j < len(this.cfgs); j++ {
			cfgDataJ := any(this.cfgs[j]).(internal.CfgData)
			if cfgDataI.GetCfgId() == cfgDataJ.GetCfgId() {
				slog.Error("duplicate id", "fileName", fileName, "id", cfgDataI.GetCfgId())
			}
		}
	}
}
