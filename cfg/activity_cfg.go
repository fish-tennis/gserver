package cfg

import (
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

var (
	_activityCfgLoader = Register(func() any {
		return new(ActivityCfgMgr)
	}, Mid+1) // 任务配置加载完,再加载活动配置
)

// 活动配置数据管理
type ActivityCfgMgr struct {
	Activities            *DataMap[*pb.ActivityCfg] `cfg:"activitycfg.csv"`
	ExchangeIdsByActivity map[int32]int32           // map[ExchangeId]ActivityId
}

// singleton
func GetActivityCfgMgr() *ActivityCfgMgr {
	return _activityCfgLoader.Load().(*ActivityCfgMgr)
}

func (m *ActivityCfgMgr) GetActivityCfg(cfgId int32) *pb.ActivityCfg {
	return m.Activities.GetCfg(cfgId)
}

// 获取礼包对应的活动id(如果有的话)
func (m *ActivityCfgMgr) GetActivityIdByExchangeId(exchangeId int32) int32 {
	return m.ExchangeIdsByActivity[exchangeId]
}

func (m *ActivityCfgMgr) AfterLoad() {
	m.ExchangeIdsByActivity = make(map[int32]int32)
	m.Activities.Range(func(e *pb.ActivityCfg) bool {
		// 自动关联活动兑换配置
		for _, exchangeId := range e.GetExchangeIds() {
			exchangeCfg := GetTemplateCfgMgr().GetExchangeCfg(exchangeId)
			if exchangeCfg == nil {
				logger.Error("exchangeCfg nil %v", exchangeId)
				return true
			}
			m.ExchangeIdsByActivity[exchangeId] = e.GetCfgId()
		}
		return true
	})
}
