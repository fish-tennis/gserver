package gameplayer

import (
	"fmt"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"testing"
)

func TestSaveable(t *testing.T) {
	util.InitIdGenerator(1)
	InitPlayerComponentMap()
	cfg.GetItemCfgMgr().Load("./../cfgdata/itemcfg.json")

	player := CreateTempPlayer(1, 1)
	// 明文保存的proto
	baseInfo := player.GetComponentByName("BaseInfo").(*BaseInfo)
	baseInfo.IncExp(1001)
	saveData,err := internal.GetComponentSaveData(baseInfo)
	if err != nil {
		t.Error(err)
	}
	t.Log(fmt.Sprintf("%v", saveData))

	// 序列化保存的proto
	money := player.GetComponentByName("Money").(*Money)
	money.IncCoin(10)
	money.IncDiamond(100)
	saveData,err = internal.GetComponentSaveData(money)
	if err != nil {
		t.Error(err)
	}
	t.Log(fmt.Sprintf("%v", saveData))

	// value是子模块的组合
	bag := player.GetComponentByName("Bag").(*Bag)
	for i := 0; i < 3; i++ {
		bag.AddItem(int32(i+1), int32((i+1)*10))
	}
	saveData,err = internal.GetComponentSaveData(bag)
	if err != nil {
		t.Error(err)
	}
	t.Log(fmt.Sprintf("%v", saveData))

	// value是子模块的组合
	quest := player.GetComponentByName("Quest").(*Quest)
	quest.Quests.Add(&pb.QuestData{
		CfgId: 1,
		Progress: 0,
	})
	quest.Quests.Add(&pb.QuestData{
		CfgId: 2,
		Progress: 1,
	})
	quest.Finished.Add(3)
	quest.Finished.Add(4)
	saveData,err = internal.GetComponentSaveData(quest)
	if err != nil {
		t.Error(err)
	}
	t.Log(fmt.Sprintf("%v", saveData))
}
