package cfg

import (
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
	"testing"
	"time"
)

func TestCfgLoad(t *testing.T) {
	dir := "./../cfgdata/"
	//LoadAllCfgs(dir, LoadCfgFilter)
	err := Load(dir, Process, nil)
	if err != nil {
		t.Fatal(err)
	}
	Quests.Range(func(e *pb.QuestCfg) bool {
		slog.Info("QuestCfg", "CfgId", e.CfgId, "Conditions", e.Conditions, "Progress", e.Progress, "Properties", e.Properties)
		return true
	})
	ActivityCfgs.Range(func(e *pb.ActivityCfg) bool {
		slog.Info("ActivityCfg", "CfgId", e.CfgId, "QuestIds", e.QuestIds, "ExchangeIds", e.ExchangeIds)
		return true
	})
	time.Sleep(time.Second)
}
