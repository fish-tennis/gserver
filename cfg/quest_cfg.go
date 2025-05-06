package cfg

import (
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
)

var (
	_questCfgLoader = Register(func() any {
		return new(QuestCfgMgr)
	}, Mid)
)

// 任务配置数据管理
type QuestCfgMgr struct {
	*DataMap[*pb.QuestCfg] `cfg:"questcfg.csv"`
}

// singleton
func GetQuestCfgMgr() *QuestCfgMgr {
	return _questCfgLoader.Load().(*QuestCfgMgr)
}

func (m *QuestCfgMgr) GetQuestCfg(cfgId int32) *pb.QuestCfg {
	return m.cfgs[cfgId]
}

func (m *QuestCfgMgr) AfterLoad() {
	templateCfg := GetTemplateCfgMgr()
	m.Range(func(e *pb.QuestCfg) bool {
		e.Conditions = templateCfg.convertConditionCfgs(e.ConditionTemplates)
		if e.ProgressTemplate == nil {
			slog.Error("ProgressTemplate nil", "QuestCfgId", e.GetCfgId())
			return true
		}
		e.Progress = templateCfg.convertProgressCfg(e.ProgressTemplate)
		return true
	})
}
