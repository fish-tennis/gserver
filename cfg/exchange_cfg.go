package cfg

import "github.com/fish-tennis/gserver/pb"

func init() {
	register.ExchangeCfgsProcess = exchangeAfterLoad
}

func exchangeAfterLoad(mgr *DataMap[*pb.ExchangeCfg]) error {
	mgr.Range(func(e *pb.ExchangeCfg) bool {
		e.Conditions = convertConditionCfgs(e.ConditionTemplates)
		return true
	})
	return nil
}
