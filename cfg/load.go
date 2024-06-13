package cfg

import (
	"cmp"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/tool"
	"slices"
)

var (
	// 配置数据加载接口注册表
	_cfgLoaders []*CfgLoaderOption
)

type CfgLoader interface {
	Load(fileName string, loaderOption *CfgLoaderOption) error
}

type CfgLoaderOption struct {
	Loader   CfgLoader
	FileName string
	// 加载顺序,数值小的,先执行
	// 因为有的数据可能有依赖关系
	Order     int
	CsvOption *tool.CsvOption // csv配置文件才需要
}

// 注册配置数据加载接口
func RegisterCfgLoader(loaderOpt *CfgLoaderOption) {
	if loaderOpt.CsvOption == nil {
		// 默认csv设置
		loaderOpt.CsvOption = &tool.CsvOption{
			DataBeginRowIndex: 1,
		}
	}
	_cfgLoaders = append(_cfgLoaders, loaderOpt)
	slices.SortStableFunc(_cfgLoaders, func(a, b *CfgLoaderOption) int {
		if a.Order == b.Order {
			return cmp.Compare(a.FileName, b.FileName)
		}
		return cmp.Compare(a.Order, b.Order)
	})
}

func LoadAllCfgs(dir string, progressMgr *internal.ProgressMgr, conditionMgr *internal.ConditionMgr) bool {
	if dir != "" && dir[len(dir)-1] != '/' {
		dir = dir + "/"
	}
	GetQuestCfgMgr().SetProgressMgr(progressMgr)
	GetQuestCfgMgr().SetConditionMgr(conditionMgr)
	GetActivityCfgMgr().SetProgressMgr(progressMgr)
	GetActivityCfgMgr().SetConditionMgr(conditionMgr)
	for _, loaderOpt := range _cfgLoaders {
		err := loaderOpt.Loader.Load(dir+loaderOpt.FileName, loaderOpt)
		if err != nil {
			logger.Error("load %v err", loaderOpt.FileName)
			return false
		}
	}
	return true
}
