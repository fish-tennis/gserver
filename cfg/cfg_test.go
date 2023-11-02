package cfg

import (
	"testing"
)

func TestQuestCfgLoad(t *testing.T) {
	dir := "./../cfgdata/"
	GetQuestCfgMgr().Load(dir + "questcfg.json")
	t.Logf("%v", GetQuestCfgMgr().cfgs)
	GetLevelCfgMgr().Load(dir + "levelcfg.csv")
	t.Logf("%v", GetLevelCfgMgr().needExps)
	GetItemCfgMgr().Load(dir + "itemcfg.json")
	t.Logf("%v", GetItemCfgMgr().cfgs)
}
