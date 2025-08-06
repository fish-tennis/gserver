package cfg

import "github.com/fish-tennis/gserver/pb"

func exchangeAfterLoad(mgr any, mgrName, messageName, fileName string) {
	exchanges := mgr.(*DataMap[*pb.ExchangeCfg])
	exchanges.Range(func(e *pb.ExchangeCfg) bool {
		e.Conditions = convertConditionCfgs(e.ConditionTemplates)
		return true
	})
}
