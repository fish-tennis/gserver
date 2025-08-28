package cfg

import (
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
	"testing"
	"time"
)

func TestCfgLoad(t *testing.T) {
	dir := "./../cfgdata/"
	err := Load(dir, nil)
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
	ConditionTemplateCfgs.Range(func(e *pb.ConditionTemplateCfg) bool {
		slog.Info("ConditionTemplateCfg", "CfgId", e.CfgId, "Conditions", e)
		return true
	})
	time.Sleep(time.Second)
}
