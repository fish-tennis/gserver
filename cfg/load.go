package cfg

import (
	"cmp"
	"github.com/fish-tennis/csv"
	"log/slog"
	"reflect"
	"slices"
	"sync/atomic"
)

var (
	// 配置数据加载接口注册表
	_cfgLoaders []*CfgLoaderOption
)

type CfgLoader interface {
	Load(fileName string, loaderOption *CfgLoaderOption) error
}

type CfgAfterLoader interface {
	AfterLoad()
}

type CfgLoaderOption struct {
	FileName string

	// 加载顺序,数值小的,先执行
	// 因为有的数据可能有依赖关系
	Order     int
	CsvOption *csv.CsvOption // csv配置文件才需要

	Value atomic.Value // 原子Value,用于热更新时的并发保护
}

// 注册配置数据加载接口
func RegisterCfgLoader(cfgLoader CfgLoader, loaderOpt *CfgLoaderOption) *CfgLoaderOption {
	if loaderOpt.CsvOption == nil {
		// 默认csv设置
		loaderOpt.CsvOption = &DefaultCsvOption
	}
	loaderOpt.Value.Store(cfgLoader)
	_cfgLoaders = append(_cfgLoaders, loaderOpt)
	slices.SortStableFunc(_cfgLoaders, func(a, b *CfgLoaderOption) int {
		if a.Order == b.Order {
			return cmp.Compare(a.FileName, b.FileName)
		}
		return cmp.Compare(a.Order, b.Order)
	})
	return loaderOpt
}

// 加载所有注册的数据,支持热更新
func LoadAllCfgs(dir string) bool {
	if dir != "" && dir[len(dir)-1] != '/' {
		dir = dir + "/"
	}
	for _, loaderOpt := range _cfgLoaders {
		// 创建临时对象加载数据
		loadingLoader := reflect.New(reflect.TypeOf(loaderOpt.Value.Load()).Elem()).Interface().(CfgLoader)
		err := loadingLoader.Load(dir+loaderOpt.FileName, loaderOpt)
		if err != nil {
			slog.Error("LoadAllCfgs err", "FileName", loaderOpt.FileName)
			return false
		}
		// 需要预处理数据的
		if afterLoader, ok := loadingLoader.(CfgAfterLoader); ok {
			afterLoader.AfterLoad()
		}
		// 加载完成后,进行原子赋值
		loaderOpt.Value.Store(loadingLoader)
	}
	return true
}
