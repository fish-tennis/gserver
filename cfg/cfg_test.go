package cfg

import "testing"

func TestQuestCfgLoad(t *testing.T) {
	GetQuestCfgMgr().Load("./../cfgdata/questcfg.json")
	t.Logf("%v", GetQuestCfgMgr().cfgs)
	GetLevelCfgMgr().Load("./../cfgdata/levelcfg.csv")
	t.Logf("%v", GetLevelCfgMgr().needExps)
	GetItemCfgMgr().Load("./../cfgdata/itemcfg.json")
	t.Logf("%v", GetItemCfgMgr().cfgs)
}
