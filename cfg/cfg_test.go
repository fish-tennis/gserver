package cfg

import (
	"github.com/fish-tennis/gserver/internal"
	"testing"
)

func TestCfgLoad(t *testing.T) {
	dir := "./../cfgdata/"
	progressMgr := internal.NewProgressMgr()
	conditionMgr := internal.NewConditionMgr()
	LoadAllCfgs(dir, progressMgr, conditionMgr)
}
