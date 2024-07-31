package game

import (
	"fmt"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/pb"
	"testing"
	"time"
)

func TestSaveable(t *testing.T) {
	gnet.SetLogLevel(-1)
	util.InitIdGenerator(1)
	InitPlayerStructAndHandler()
	progressMgr := RegisterProgressCheckers()
	conditionMgr := RegisterConditionCheckers()
	cfg.LoadAllCfgs("./../cfgdata")
	cfg.GetQuestCfgMgr().SetProgressMgr(progressMgr)
	cfg.GetQuestCfgMgr().SetConditionMgr(conditionMgr)
	cfg.GetActivityCfgMgr().SetProgressMgr(progressMgr)
	cfg.GetActivityCfgMgr().SetConditionMgr(conditionMgr)

	player := CreateTempPlayer(1, 1)
	// 明文保存的proto
	baseInfo := player.GetBaseInfo()
	baseInfo.IncExp(1001)
	saveData, err := gentity.GetComponentSaveData(baseInfo)
	if err != nil {
		t.Error(err)
	}
	t.Log(fmt.Sprintf("%v", saveData))

	// 序列化保存的proto
	money := player.GetMoney()
	money.IncCoin(10)
	money.IncDiamond(100)
	saveData, err = gentity.GetComponentSaveData(money)
	if err != nil {
		t.Error(err)
	}
	t.Log(fmt.Sprintf("%v", saveData))

	// value是子模块的组合
	bag := player.GetBag()
	for i := 0; i < 3; i++ {
		bag.AddItem(int32(i+1), int32((i+1)*10))
	}
	saveData, err = gentity.GetComponentSaveData(bag)
	if err != nil {
		t.Error(err)
	}
	t.Log(fmt.Sprintf("%v", saveData))

	// value是子模块的组合
	quest := player.GetQuest()
	questData1 := &pb.QuestData{
		CfgId:    1,
		Progress: 0,
	}
	quest.Quests.Set(questData1.CfgId, questData1)
	questData2 := &pb.QuestData{
		CfgId:    2,
		Progress: 1,
	}
	quest.Quests.Set(questData2.CfgId, questData2)
	quest.Finished.Add(3)
	quest.Finished.Add(4)
	saveData, err = gentity.GetComponentSaveData(quest)
	if err != nil {
		t.Error(err)
	}
	t.Log(fmt.Sprintf("%v", saveData))
}

func TestActivity(t *testing.T) {
	gnet.SetLogLevel(-1)
	util.InitIdGenerator(1)
	InitPlayerStructAndHandler()
	progressMgr := RegisterProgressCheckers()
	conditionMgr := RegisterConditionCheckers()
	cfg.LoadAllCfgs("./../cfgdata")
	cfg.GetQuestCfgMgr().SetProgressMgr(progressMgr)
	cfg.GetQuestCfgMgr().SetConditionMgr(conditionMgr)
	cfg.GetActivityCfgMgr().SetProgressMgr(progressMgr)
	cfg.GetActivityCfgMgr().SetConditionMgr(conditionMgr)

	playerData := &pb.PlayerData{
		XId:       1,
		Name:      "test",
		AccountId: 1,
		RegionId:  1,
	}
	player := CreatePlayer(playerData.XId, playerData.Name, playerData.AccountId, playerData.RegionId)
	activities := player.GetActivities()
	activityIds := []int32{1, 2, 3, 4, 5}
	for _, activityId := range activityIds {
		activityCfg := cfg.GetActivityCfgMgr().GetActivityCfg(activityId)
		if activityCfg == nil {
			continue
		}
		activities.AddNewActivity(activityCfg, time.Now())
	}

	eventSignIn := &pb.EventPlayerPropertyInc{
		PlayerId:      player.GetId(),
		PropertyName:  "SignIn", // 签到事件
		PropertyValue: 1,
	}
	player.FireEvent(eventSignIn)

	eventTotalPay := &pb.EventPlayerPropertyInc{
		PlayerId:      player.GetId(),
		PropertyName:  "TotalPay", //累计充值
		PropertyValue: 10,
	}
	player.FireEvent(eventTotalPay)

	eventOnlineTime := &pb.EventPlayerPropertyInc{
		PlayerId:      player.GetId(),
		PropertyName:  "OnlineMinute", //在线时长
		PropertyValue: 2,
	}
	player.FireEvent(eventOnlineTime)

	for _, activityId := range activityIds {
		activity := activities.GetActivity(activityId)
		t.Log(fmt.Sprintf("%v %v", activityId, activity.(*ActivityDefault)))
	}

	exchangeActivity := activities.GetActivity(5)
	if exchangeActivity != nil {
		if activity, ok := exchangeActivity.(*ActivityDefault); ok {
			player.GetBag().AddItem(1, 100)
			t.Log(fmt.Sprintf("item1 count:%v", player.GetBag().BagCountItem.GetItemCount(1)))
			t.Log(fmt.Sprintf("item2 count:%v", player.GetBag().BagCountItem.GetItemCount(2)))
			for i := 0; i < 3; i++ {
				activity.Exchange(1)
			}
			t.Log(fmt.Sprintf("item1 count:%v", player.GetBag().BagCountItem.GetItemCount(1)))
			t.Log(fmt.Sprintf("item2 count:%v", player.GetBag().BagCountItem.GetItemCount(2)))
		}
	}

	for i := 1; i <= 3; i++ {
		now := time.Now()
		oldDate := now.AddDate(0, 0, -i)
		t.Logf("%v %v", oldDate.String(), now.String())
		for _, activityId := range activityIds {
			activity := activities.GetActivity(activityId)
			if activity == nil {
				continue
			}
			activityDefault, ok := activity.(*ActivityDefault)
			if !ok {
				continue
			}
			// 参加活动的时间回退到i天前
			activityDefault.Base.JoinTime = int32(oldDate.Unix())
			activity.OnDateChange(oldDate, now)
			t.Log(fmt.Sprintf("%v %v", activityId, activityDefault.Base.Progresses))
		}
	}
}
