package cfg

import (
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

var (
	ExchangeIdsByActivity map[int32]int32 // map[ExchangeId]ActivityId
)

func ActivityAfterLoad(mgr any, mgrName, messageName, fileName string) {
	ExchangeIdsByActivity = make(map[int32]int32)
	activities := mgr.(*DataMap[*pb.ActivityCfg])
	activities.Range(func(e *pb.ActivityCfg) bool {
		// 自动关联活动兑换配置
		for _, exchangeId := range e.GetExchangeIds() {
			exchangeCfg := ExchangeCfgs.GetCfg(exchangeId)
			if exchangeCfg == nil {
				logger.Error("exchangeCfg nil %v", exchangeId)
				return true
			}
			ExchangeIdsByActivity[exchangeId] = e.GetCfgId()
		}
		return true
	})
}

// 获取礼包对应的活动id(如果有的话)
func GetActivityIdByExchangeId(exchangeId int32) int32 {
	return ExchangeIdsByActivity[exchangeId]
}
