package cfg

import (
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
)

func init() {
	register.QuestsProcess = questAfterLoad
}

func questAfterLoad(mgr *DataMap[*pb.QuestCfg]) error {
	mgr.Range(func(e *pb.QuestCfg) bool {
		e.Conditions = convertConditionCfgs(e.ConditionTemplates)
		if e.ProgressTemplate == nil {
			slog.Error("ProgressTemplate nil", "QuestCfgId", e.GetCfgId())
			return true
		}
		e.Progress = convertProgressCfg(e.ProgressTemplate)
		return true
	})
	return nil
}
