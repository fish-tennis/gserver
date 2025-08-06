package cfg

import (
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
)

func questAfterLoad(mgr any, mgrName, messageName, fileName string) {
	quests := mgr.(*DataMap[*pb.QuestCfg])
	quests.Range(func(e *pb.QuestCfg) bool {
		e.Conditions = convertConditionCfgs(e.ConditionTemplates)
		if e.ProgressTemplate == nil {
			slog.Error("ProgressTemplate nil", "QuestCfgId", e.GetCfgId())
			return true
		}
		e.Progress = convertProgressCfg(e.ProgressTemplate)
		return true
	})
}
