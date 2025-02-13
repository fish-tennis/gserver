package game

import (
	"context"
	"fmt"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"gopkg.in/natefinch/lumberjack.v2"
	"log/slog"
	"os"
	"testing"
	"time"
)

var (
	_mongoUri       = "mongodb://localhost:27017"
	_mongoDbName    = "test"
	_collectionName = "player"
	_redisAddrs     = []string{"127.0.0.1:6379"}
	_redisUsername  = ""
	_redisPassword  = ""
	// 如果部署的是单机版redis,则需要修改为false
	_isRedisCluster = true
)

func initRedis() {
	cache.NewRedis(_redisAddrs, _redisUsername, _redisPassword, _isRedisCluster)
}

func initLog(logFileName string, useStdOutput bool) {
	gnet.SetLogger(logger.GetLogger(), gnet.DebugLevel)
	gentity.SetLogger(logger.GetLogger(), gnet.DebugLevel)

	os.Mkdir("log", 0750)
	// 日志轮转与切割
	fileLogger := &lumberjack.Logger{
		Filename:   fmt.Sprintf("log/%v.log", logFileName),
		MaxSize:    10,
		MaxBackups: 100,
		MaxAge:     7,
		Compress:   false,
		LocalTime:  true,
	}
	// 建议使用slog
	debugLevel := &slog.LevelVar{}
	debugLevel.Set(slog.LevelDebug)
	slog.SetDefault(slog.New(logger.NewJsonHandlerWithStdOutput(fileLogger, &slog.HandlerOptions{
		AddSource: true,
		Level:     debugLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				source := a.Value.Any().(*slog.Source)
				source.Function = ""
				source.File = logger.GetShortFileName(source.File)
			}
			return a
		},
	}, useStdOutput)))
}

func initTestEnv(t *testing.T) {
	//gnet.SetLogLevel(-1)
	initLog("test", true)
	util.InitIdGenerator(1)
	InitPlayerStructAndHandler()
	AutoRegisterPlayerPacketHandler(nil)
	progressMgr := RegisterProgressCheckers()
	conditionMgr := RegisterConditionCheckers()
	cfg.LoadAllCfgs("./../cfgdata", cfg.LoadCfgFilter)
	cfg.GetQuestCfgMgr().SetProgressMgr(progressMgr)
	cfg.GetQuestCfgMgr().SetConditionMgr(conditionMgr)
	cfg.GetActivityCfgMgr().SetProgressMgr(progressMgr)
	cfg.GetActivityCfgMgr().SetConditionMgr(conditionMgr)
}

func TestSaveable(t *testing.T) {
	initTestEnv(t)

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
	bag := player.GetBags()
	for i := 0; i < 3; i++ {
		bag.AddItemById(int32(i+1), int32((i+1)*10))
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
	initTestEnv(t)

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

	eventTotalPay := &pb.EventPlayerProperty{
		PlayerId:      player.GetId(),
		PropertyName:  "TotalPay", //累充
		PropertyValue: 10,
	}
	player.FireEvent(eventTotalPay)

	eventOnlineTime := &pb.EventPlayerProperty{
		PlayerId:      player.GetId(),
		PropertyName:  "OnlineMinute", //在线时长
		PropertyValue: 2,
	}
	player.FireEvent(eventOnlineTime)

	eventFight := &pb.EventFight{
		PlayerId: player.GetId(),
		IsPvp:    true,
		IsWin:    false,
	}
	player.FireEvent(eventFight)

	eventFight = &pb.EventFight{
		PlayerId: player.GetId(),
		IsPvp:    true,
		IsWin:    true,
	}
	player.FireEvent(eventFight)

	for _, activityId := range activityIds {
		activity := activities.GetActivity(activityId).(*ActivityDefault)
		t.Log(fmt.Sprintf("%v Progresses:%v", activityId, activity.Base.Quests))
		t.Log(fmt.Sprintf("%v ExchangeRecord:%v", activityId, activity.Base.ExchangeRecord))
	}

	exchangeActivity := activities.GetActivity(1) // 每日签到
	if exchangeActivity != nil {
		activity := exchangeActivity.(*ActivityDefault)
		for _, exchangeId := range activity.GetActivityCfg().GetExchangeIds() {
			activity.Exchange(exchangeId)
		}
		t.Log(fmt.Sprintf("%v ExchangeRecord:%v", 1, exchangeActivity.(*ActivityDefault).Base.ExchangeRecord))
	}
	exchangeActivity = activities.GetActivity(5) // 活动商店
	if exchangeActivity != nil {
		if activity, ok := exchangeActivity.(*ActivityDefault); ok {
			player.GetBags().AddItemById(1, 100)
			player.GetBags().AddItemById(2, 100)
			t.Log(fmt.Sprintf("item1 count:%v", player.GetBags().GetItemCount(1)))
			t.Log(fmt.Sprintf("item2 count:%v", player.GetBags().GetItemCount(2)))
			t.Log(fmt.Sprintf("item3 count:%v", player.GetBags().GetItemCount(3)))
			t.Log(fmt.Sprintf("item4 count:%v", player.GetBags().GetItemCount(4)))
			for i := 0; i < 3; i++ {
				for _, exchangeId := range activity.GetActivityCfg().GetExchangeIds() {
					activity.Exchange(exchangeId)
				}
			}
			t.Log(fmt.Sprintf("item1 count:%v", player.GetBags().GetItemCount(1)))
			t.Log(fmt.Sprintf("item2 count:%v", player.GetBags().GetItemCount(2)))
			t.Log(fmt.Sprintf("item3 count:%v", player.GetBags().GetItemCount(3)))
			t.Log(fmt.Sprintf("item4 count:%v", player.GetBags().GetItemCount(4)))
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
			t.Log(fmt.Sprintf("%v %v", activityId, activityDefault.Base.Quests))
		}
	}
}

type dummyPlayerMgr struct {
	internal.PlayerMgr
}

func (d *dummyPlayerMgr) GetPlayer(playerId int64) internal.IPlayer {
	return nil
}

func (d *dummyPlayerMgr) AddPlayer(player internal.IPlayer) {}

func (d *dummyPlayerMgr) RemovePlayer(player internal.IPlayer) {}

func TestBags(t *testing.T) {
	initTestEnv(t)
	initRedis()

	ctx, cancel := context.WithCancel(context.Background())
	server := internal.NewBaseServer(ctx, "test", "")
	gentity.SetApplication(server)
	SetPlayerMgr(new(dummyPlayerMgr))

	playerData := &pb.PlayerData{
		XId:       1,
		Name:      "test",
		AccountId: 1,
		RegionId:  1,
	}
	player := CreatePlayer(playerData.XId, playerData.Name, playerData.AccountId, playerData.RegionId)
	bags := player.GetBags()

	addItemArgs := []*pb.AddItemArg{
		{
			CfgId: 1,
			Num:   1,
		},
		{
			CfgId: 2,
			Num:   10,
		},
		{
			CfgId: 3,
			Num:   2,
		},
		{
			CfgId:    1,
			Num:      1,
			TimeType: int32(pb.TimeType_TimeType_Timestamp),
			Timeout:  int32(time.Now().Unix()) + 1, // 1秒后过期
		},
		{
			CfgId:    4,
			Num:      1,
			TimeType: int32(pb.TimeType_TimeType_Timestamp),
			Timeout:  int32(time.Now().Unix()) + 2, // 2秒后过期
		},
	}
	bags.AddItems(addItemArgs)

	player.RunRoutine()
	// 转到玩家协程中去处理
	player.OnRecvPacket(gnet.NewProtoPacket(gnet.PacketCommand(pb.CmdServer_Cmd_PlayerEntryGameOk), &pb.PlayerEntryGameOk{
		IsReconnect: false,
	}))
	time.Sleep(time.Second * 5)
	cancel()
}
