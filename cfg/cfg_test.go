package cfg

import (
	"fmt"
	"github.com/fish-tennis/gserver/internal"
	"log/slog"
	"testing"
	"time"
)

func TestCfgLoad(t *testing.T) {
	dir := "./../cfgdata/"
	progressMgr := internal.NewProgressMgr()
	conditionMgr := internal.NewConditionMgr()
	LoadAllCfgs(dir, LoadCfgFilter)
	GetQuestCfgMgr().SetProgressMgr(progressMgr)
	GetQuestCfgMgr().SetConditionMgr(conditionMgr)
	GetActivityCfgMgr().SetProgressMgr(progressMgr)
	GetActivityCfgMgr().SetConditionMgr(conditionMgr)
	GetQuestCfgMgr().Range(func(e *QuestCfg) bool {
		slog.Info("QuestCfg", "Conditions", e.Conditions, "ProgressCfg", e.ProgressCfg, "Properties", e.ProgressCfg.Properties)
		return true
	})
	GetActivityCfgMgr().Range(func(e *ActivityCfg) bool {
		slog.Info("ActivityCfg", "CfgId", e.CfgId, "Quests", e.Quests, "Properties", e.Properties)
		for i, exchange := range e.Exchanges {
			slog.Info(fmt.Sprintf("Exchanges[%v]", i), "exchange", exchange)
		}
		return true
	})
	time.Sleep(time.Second)
}
