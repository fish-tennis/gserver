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
		// 任务不能同时没有进度和收集物品
		if e.ProgressTemplate == nil && len(e.GetCollects()) == 0 {
			slog.Info("QuestCfgErr", "QuestCfgId", e.GetCfgId())
			return true
		}
		if e.ProgressTemplate != nil {
			e.Progress = convertProgressCfg(e.ProgressTemplate)
		}
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
