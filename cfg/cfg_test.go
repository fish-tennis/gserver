package cfg

import (
	"github.com/fish-tennis/gserver/internal"
	"testing"
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
}
