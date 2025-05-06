package cfg

import (
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
	"testing"
	"time"
)

func TestCfgLoad(t *testing.T) {
	dir := "./../cfgdata/"
	LoadAllCfgs(dir, LoadCfgFilter)
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
