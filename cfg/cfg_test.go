package cfg

import "testing"

func TestQuestCfgLoad(t *testing.T) {
	GetQuestCfgMgr().Load("./../cfgdata/questcfg.json")
}
