package cfg

import (
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

var (
	ExchangeIdsByActivity map[int32]int32 // map[ExchangeId]ActivityId
)

func init() {
	register.ActivityCfgsProcess = activityAfterLoad
}

func activityAfterLoad(mgr *DataMap[*pb.ActivityCfg]) error {
	tmpExchangeIdsByActivity := make(map[int32]int32)
	mgr.Range(func(e *pb.ActivityCfg) bool {
		// 自动关联活动兑换配置
		for _, exchangeId := range e.GetExchangeIds() {
			exchangeCfg := ExchangeCfgs.GetCfg(exchangeId)
			if exchangeCfg == nil {
				logger.Error("exchangeCfg nil %v", exchangeId)
				return true
			}
			tmpExchangeIdsByActivity[exchangeId] = e.GetCfgId()
		}
		return true
	})
	ExchangeIdsByActivity = tmpExchangeIdsByActivity
	return nil
}

// 获取礼包对应的活动id(如果有的话)
func GetActivityIdByExchangeId(exchangeId int32) int32 {
	return ExchangeIdsByActivity[exchangeId]
}
