package gameplayer

import (
	"fmt"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"testing"
)

func TestSaveable(t *testing.T) {
	util.InitIdGenerator(1)
	InitPlayerComponentMap()

	player := CreateTempPlayer(1, 1)
	// 明文保存的proto
	baseInfo := player.GetComponentByName("BaseInfo").(*BaseInfo)
	baseInfo.IncExp(1001)
	saveData,err := internal.SaveSaveable_New(baseInfo)
	if err != nil {
		t.Error(err)
	}
	t.Log(fmt.Sprintf("%v", saveData))

	// 序列化保存的proto
	money := player.GetComponentByName("Money").(*Money)
	money.IncCoin(10)
	money.IncDiamond(100)
	saveData,err = internal.SaveSaveable_New(money)
	if err != nil {
		t.Error(err)
	}
	t.Log(fmt.Sprintf("%v", saveData))

	// 基础类型的map
	countBag := player.GetComponentByName("BagCountItem").(*BagCountItem)
	for i := 0; i < 3; i++ {
		countBag.AddItem(int32(i+1), int32((i+1)*10))
	}
	saveData,err = internal.SaveSaveable_New(countBag)
	if err != nil {
		t.Error(err)
	}
	t.Log(fmt.Sprintf("%v", saveData))

	// value是proto的map
	uniqueBag := player.GetComponentByName("BagUniqueItem").(*BagUniqueItem)
	for i := 0; i < 3; i++ {
		uniqueBag.AddUniqueItem(&pb.UniqueItem{
			UniqueId: util.GenUniqueId(),
			CfgId: int32(i+1),
		})
	}
	saveData,err = internal.SaveSaveable_New(uniqueBag)
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
	saveData,_,err = internal.SaveCompositeSaveable_New(quest, true)
	if err != nil {
		t.Error(err)
	}
	t.Log(fmt.Sprintf("%v", saveData))
}
