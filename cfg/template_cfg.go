package cfg

import (
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
	"maps"
	"slices"
)

var (
	_templateCfgLoader = Register(func() any {
		return new(TemplateCfgMgr)
	}, First+1)
)

// 模板配置数据管理
type TemplateCfgMgr struct {
	ConditionTemplates *DataMap[*pb.ConditionTemplateCfg] `cfg:"condition_template.csv"`
	ProgressTemplates  *DataMap[*pb.ProgressTemplateCfg]  `cfg:"progress_template.csv"`
	Exchanges          *DataMap[*pb.ExchangeCfg]          `cfg:"exchange.csv"`
}

// singleton
func GetTemplateCfgMgr() *TemplateCfgMgr {
	return _templateCfgLoader.Load().(*TemplateCfgMgr)
}

func (m *TemplateCfgMgr) GetExchangeCfg(cfgId int32) *pb.ExchangeCfg {
	return m.Exchanges.GetCfg(cfgId)
}

func (m *TemplateCfgMgr) AfterLoad() {
	m.Exchanges.Range(func(e *pb.ExchangeCfg) bool {
		e.Conditions = m.convertConditionCfgs(e.ConditionTemplates)
		return true
	})
}

func (m *TemplateCfgMgr) convertConditionCfg(cfgArg *pb.CfgArgs) *pb.ConditionCfg {
	conditionTemplate := m.ConditionTemplates.GetCfg(cfgArg.CfgId)
	if conditionTemplate == nil {
		return nil
	}
	return &pb.ConditionCfg{
		Type:       conditionTemplate.Type,
		Key:        conditionTemplate.Key,
		Op:         conditionTemplate.Op,
		Args:       slices.Clone(cfgArg.Args),
		Properties: maps.Clone(conditionTemplate.Properties),
	}
}

func (m *TemplateCfgMgr) convertConditionCfgs(cfgArgs []*pb.CfgArgs) []*pb.ConditionCfg {
	var conditions []*pb.ConditionCfg
	for _, cfgArg := range cfgArgs {
		condition := m.convertConditionCfg(cfgArg)
		if condition == nil {
			slog.Error("condition nil", "cfgArg", cfgArg)
			continue
		}
		conditions = append(conditions, condition)
	}
	return conditions
}

func (m *TemplateCfgMgr) convertProgressCfg(cfgArg *pb.CfgArg) *pb.ProgressCfg {
	progressTemplate := m.ProgressTemplates.GetCfg(cfgArg.CfgId)
	if progressTemplate == nil {
		return nil
	}
	return &pb.ProgressCfg{
		Type:       progressTemplate.Type,
		NeedInit:   progressTemplate.NeedInit,
		Event:      progressTemplate.Event,
		EventField: progressTemplate.EventField,
		Total:      cfgArg.Arg,
		Properties: maps.Clone(progressTemplate.Properties),
	}
}

func (m *TemplateCfgMgr) convertProgressCfgs(cfgArgs []*pb.CfgArg) []*pb.ProgressCfg {
	var progressCfgs []*pb.ProgressCfg
	for _, cfgArg := range cfgArgs {
		progress := m.convertProgressCfg(cfgArg)
		if progress == nil {
			slog.Error("progress nil", "cfgArg", cfgArg)
			continue
		}
		progressCfgs = append(progressCfgs, progress)
	}
	return progressCfgs
}
