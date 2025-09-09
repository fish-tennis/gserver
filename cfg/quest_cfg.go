package cfg

import (
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
)

var (
	QuestsByLevel map[int32]*DataMap[*pb.QuestCfg] // 按玩家等级限制的索引
	QuestsDay     *DataMap[*pb.QuestCfg]           // 日常刷新的任务
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
	QuestsByLevel = mgr.CreateIndexInt32(func(e *pb.QuestCfg) int32 {
		return e.GetPlayerLevel()
	})
	QuestsDay = mgr.CreateSubset(func(e *pb.QuestCfg) bool {
		return e.GetRefreshType() == int32(pb.RefreshType_RefreshType_Day)
	})
	return nil
}
