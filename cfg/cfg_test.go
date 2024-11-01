package cfg

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
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
	GetQuestCfgMgr().Range(func(e *pb.QuestCfg) bool {
		slog.Info("QuestCfg", "CfgId", e.CfgId, "Conditions", e.Conditions, "Progress", e.Progress, "Properties", e.Properties)
		return true
	})
	GetActivityCfgMgr().Range(func(e *pb.ActivityCfg) bool {
		slog.Info("ActivityCfg", "CfgId", e.CfgId, "QuestIds", e.QuestIds, "ExchangeIds", e.ExchangeIds)
		return true
	})
	time.Sleep(time.Second)
}
