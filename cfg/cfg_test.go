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
	//bytes, err := json.Marshal(GetQuestCfgMgr().cfgs)
	//if err != nil {
	//	t.Logf("%v", err.Error())
	//	return
	//}
	//t.Logf("%v", string(bytes))
}
