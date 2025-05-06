package cfg

import (
	"github.com/fish-tennis/gserver/pb"
)

var (
	_activityCfgLoader = Register(func() any {
		return new(ActivityCfgMgr)
	}, Mid+1) // 任务配置加载完,再加载活动配置
)

// 活动配置数据管理
type ActivityCfgMgr struct {
	*DataMap[*pb.ActivityCfg] `cfg:"activitycfg.csv"`
}

// singleton
func GetActivityCfgMgr() *ActivityCfgMgr {
	return _activityCfgLoader.Load().(*ActivityCfgMgr)
}

func (m *ActivityCfgMgr) GetActivityCfg(cfgId int32) *pb.ActivityCfg {
	return m.cfgs[cfgId]
}
