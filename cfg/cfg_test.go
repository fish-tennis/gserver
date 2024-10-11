package cfg

import (
	"github.com/fish-tennis/gserver/internal"
	"log/slog"
	"testing"
	"time"
)

func TestCfgLoad(t *testing.T) {
	dir := "./../cfgdata/"
	progressMgr := internal.NewProgressMgr()
	conditionMgr := internal.NewConditionMgr()
	LoadAllCfgs(dir)
	GetQuestCfgMgr().SetProgressMgr(progressMgr)
	GetQuestCfgMgr().SetConditionMgr(conditionMgr)
	GetActivityCfgMgr().SetProgressMgr(progressMgr)
	GetActivityCfgMgr().SetConditionMgr(conditionMgr)
	GetQuestCfgMgr().Range(func(e *QuestCfg) bool {
		slog.Info("QuestCfg", "ProgressCfg", e.ProgressCfg, "map", e.ProgressCfg.Properties)
		return true
	})
	time.Sleep(time.Second)
}
