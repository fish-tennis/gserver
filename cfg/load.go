package cfg

import (
	"cmp"
	"github.com/fish-tennis/csv"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"reflect"
	"slices"
	"sync/atomic"
)

const (
	Tag = "cfg"
)

var (
	// 配置数据加载接口注册表
	_cfgLoaders []*LoaderOption
)

type Order int

const (
	// 最先加载
	First Order = 0
	// 中间
	Mid Order = 100
	// 最后加载
	Last Order = 10000
)

type Loader interface {
	Load(fileName string, loaderOption *LoaderOption) error
}

type AfterLoader interface {
	AfterLoad()
}

type LoaderOption struct {
	// 加载顺序,数值小的,先执行
	// 因为有的数据可能有依赖关系
	Order     int
	CsvOption *csv.CsvOption // csv配置文件才需要

	CfgMgrCtor func() any
	value      atomic.Value // 原子Value,用于热更新时的并发保护
}

// 注册配置数据
func Register(cfgMgrCtorLoader func() any, order Order) *atomic.Value {
	loaderOpt := &LoaderOption{
		Order:      int(order),
		CfgMgrCtor: cfgMgrCtorLoader,
		CsvOption:  &DefaultCsvOption,
	}
	cfgMgr := loaderOpt.CfgMgrCtor()
	loaderOpt.value.Store(cfgMgr)
	_cfgLoaders = append(_cfgLoaders, loaderOpt)
	slices.SortStableFunc(_cfgLoaders, func(a, b *LoaderOption) int {
		return cmp.Compare(a.Order, b.Order)
	})
	return &loaderOpt.value
}

func loadCfg(mgr any, dir string, loaderOpt *LoaderOption, loadFilter func(f string) bool) (loaded bool, err error) {
	objTyp := reflect.TypeOf(mgr)
	if objTyp.Kind() == reflect.Pointer {
		objTyp = objTyp.Elem()
	}
	objVal := reflect.ValueOf(mgr)
	if objVal.Kind() == reflect.Pointer {
		objVal = objVal.Elem()
	}
	for i := 0; i < objTyp.NumField(); i++ {
		fieldStruct := objTyp.Field(i)
		fileName := fieldStruct.Tag.Get(Tag)
		if fileName == "" {
			continue
		}
		// 防止重复加载
		if !loadFilter(fileName) {
			continue
		}
		fieldVal := objVal.Field(i)
		fieldTyp := fieldStruct.Type
		if fieldTyp.Kind() == reflect.Pointer {
			if !fieldVal.CanSet() {
				slog.Error("loadCfg init field err", "fieldName", fieldStruct.Name)
				continue
			}
			fieldVal.Set(reflect.New(fieldTyp.Elem()))
		}
		fieldAny := fieldVal.Interface()
		// map[K]V slice[E]
		if cfgLoader, ok := fieldAny.(Loader); ok {
			err = cfgLoader.Load(dir+fileName, loaderOpt)
			if err != nil {
				slog.Error("loadCfg err", "fileName", fileName)
				return
			}
		} else if _, ok := fieldAny.(proto.Message); ok {
			err = csv.ReadCsvFileObject(dir+fileName, fieldAny, loaderOpt.CsvOption)
			if err != nil {
				slog.Error("loadCfg err", "fileName", fileName)
				return
			}
		} else {
			slog.Error("loadCfg not support field", "fieldName", fieldStruct.Name, "fileName", fileName)
			continue
		}
		loaded = true
		slog.Info("loadCfg", "fileName", fileName)
	}
	return
}

// 加载所有注册的数据,支持热更新
func LoadAllCfgs(dir string, loadFilter func(f string) bool) bool {
	if dir != "" && dir[len(dir)-1] != '/' {
		dir = dir + "/"
	}
	hasError := false
	for _, loaderOpt := range _cfgLoaders {
		// 创建临时对象加载数据
		tmpCfgMgr := loaderOpt.CfgMgrCtor()
		loaded, err := loadCfg(tmpCfgMgr, dir, loaderOpt, loadFilter)
		if err != nil {
			hasError = true
			continue
		}
		if !loaded {
			continue
		}
		// 需要预处理数据的
		if afterLoader, ok := tmpCfgMgr.(AfterLoader); ok {
			afterLoader.AfterLoad()
		}
		// 加载完成后,进行原子赋值
		loaderOpt.value.Store(tmpCfgMgr)
	}
	return !hasError
}

// TODO: 配置数据热更新时,过滤没数据变化的文件,由具体项目业务层实现
func LoadCfgFilter(fileName string) bool {
	return true
}
