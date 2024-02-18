package pb

// 手动扩展pb代码的接口

// 实现ProgressHolder接口
func (x *QuestData) SetProgress(progress int32) {
	if x != nil {
		x.Progress = progress
	}
}

// 实现ProgressHolder接口
func (x *ActivityProgressData) SetProgress(progress int32) {
	if x != nil {
		x.Progress = progress
	}
}

func (x *BaseActivityCfg) GetExchangeCfg(cfgId int32) *ExchangeCfg {
	if x != nil {
		for _,cfg := range x.Exchanges {
			if cfg.CfgId == cfgId {
				return cfg
			}
		}
	}
	return nil
}