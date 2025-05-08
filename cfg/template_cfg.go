package cfg

import (
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"log/slog"
	"maps"
	"slices"
)

// 模板配置的需求场景:
// 在一个分工比较细的团队里,往往由专门的策划人员来配置数据(各种excel表)
// 但是ConditionCfg和ProgressCfg的配置较复杂且动态配置项多,因此把复杂的部分分解成模板配置
// 如condition_template.csv和progress_template.csv里配置除了参数值之外的配置项,这2个表可以由程序来配置
// 其他表要配置条件和进度,只需要配置对应的模板id和参数即可,就可以由策划人员轻松配置了

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

// 条件模板id+values,转换成ConditionCfg对象
func (m *TemplateCfgMgr) convertConditionCfg(cfgArg *pb.CfgArgs) *pb.ConditionCfg {
	conditionTemplate := m.ConditionTemplates.GetCfg(cfgArg.CfgId)
	if conditionTemplate == nil {
		return nil
	}
	return &pb.ConditionCfg{
		Type:       conditionTemplate.Type,
		Key:        conditionTemplate.Key,
		Op:         conditionTemplate.Op,
		Values:     slices.Clone(cfgArg.Args),
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

// 进度模板配置id+进度值,转换成ProgressCfg对象
func (m *TemplateCfgMgr) convertProgressCfg(cfgArg *pb.CfgArg) *pb.ProgressCfg {
	progressTemplate := m.ProgressTemplates.GetCfg(cfgArg.CfgId)
	if progressTemplate == nil {
		return nil
	}
	progressTemplate = util.CloneMessage(progressTemplate)
	return &pb.ProgressCfg{
		Type:              progressTemplate.Type,
		Total:             cfgArg.Arg,
		NeedInit:          progressTemplate.NeedInit,
		Event:             progressTemplate.Event,
		ProgressField:     progressTemplate.ProgressField,
		IntEventFields:    progressTemplate.IntEventFields,
		StringEventFields: progressTemplate.StringEventFields,
		Properties:        progressTemplate.Properties,
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
