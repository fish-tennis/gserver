package game

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gserver/cfg"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
	"slices"
	"time"
)

func init() {
	// 自动注册默认活动模板构造函数
	_activityTemplateCtorMap["default"] = func(activities ActivityMgr, activityCfg *pb.ActivityCfg, args interface{}) Activity {
		t := args.(time.Time)
		newActivity := &ActivityDefault{
			Base: &pb.ActivityDefaultBaseData{
				JoinTime: int32(t.Unix()),
			},
		}
		newActivity.Parent = activities.(gentity.MapDirtyMark)
		newActivity.MapKey = activityCfg.CfgId
		newActivity.Id = activityCfg.CfgId
		newActivity.Activities = activities.(*Activities)
		newActivity.Reset()
		return newActivity
	}
}

// 默认活动模板,支持常见的简单活动
type ActivityDefault struct {
	ChildActivity
	// 子活动的保存数据必须是一个整体,无法再细分,因为gentity目前只支持2层结构(Activities是第1层,子活动是第2层)
	Base *pb.ActivityDefaultBaseData `db:"Base"`
}

func (a *ActivityDefault) GetQuest(cfgId int32) *pb.ActivityQuestData {
	if a.Base.Quests == nil {
		return nil
	}
	return a.Base.Quests[cfgId]
}

// 添加一个活动任务
func (a *ActivityDefault) AddQuest(questCfg *pb.QuestCfg) *pb.ActivityQuestData {
	if !cfg.GetActivityCfgMgr().GetConditionMgr().CheckConditions(a, questCfg.Conditions) {
		return nil
	}
	if questCfg.Progress == nil {
		return nil
	}
	if a.Base.Quests == nil {
		a.Base.Quests = make(map[int32]*pb.ActivityQuestData)
	}
	progress := &pb.ActivityQuestData{
		CfgId: questCfg.CfgId,
	}
	if questCfg.Progress.NeedInit {
		cfg.GetActivityCfgMgr().GetProgressMgr().InitProgress(a, questCfg.Progress, progress)
	}
	a.Base.Quests[progress.CfgId] = progress
	a.SetDirty()
	a.Activities.GetPlayer().progressEventMapping.addProgress(questCfg.Progress, &ActivityQuestDataWrapper{
		ActivityQuestData: progress,
		ActivityId:        a.GetId(),
	})
	return progress
}

func (a *ActivityDefault) OnDataLoad() {
	// 把已有任务加入到进度更新映射表中
	for _, questData := range a.Base.Quests {
		questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(questData.GetCfgId())
		if questCfg == nil {
			logger.Error("questCfg nil %v", questData.GetCfgId())
			continue
		}
		a.Activities.GetPlayer().progressEventMapping.addProgress(questCfg.Progress, &ActivityQuestDataWrapper{
			ActivityQuestData: questData,
			ActivityId:        a.GetId(),
		})
	}
}

func (a *ActivityDefault) OnEvent(event interface{}) {
	activityCfg := a.GetActivityCfg()
	if activityCfg == nil {
		return
	}
	switch e := event.(type) {
	case *EventDateChange:
		a.OnDateChange(e.OldDate, e.CurDate)
		return
	}
}

func (a *ActivityDefault) OnDateChange(oldDate time.Time, curDate time.Time) {
	activityCfg := a.GetActivityCfg()
	if activityCfg == nil {
		logger.Debug("%v OnDateChange activityCfg nil %v", a.Activities.GetPlayer().GetId(), a.GetId())
		return
	}
	// 每日刷新
	if activityCfg.RefreshType == int32(pb.RefreshType_RefreshType_Day) {
		a.Reset()
	}
}

// 重置数据
func (a *ActivityDefault) Reset() {
	for _, progress := range a.Base.Quests {
		questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(progress.GetCfgId())
		if questCfg == nil {
			continue
		}
		a.Activities.GetPlayer().progressEventMapping.removeProgress(questCfg.Progress, progress.GetCfgId())
	}
	a.Base.Quests = nil
	activityCfg := a.GetActivityCfg()
	for _, questId := range activityCfg.QuestIds {
		questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(questId)
		if questCfg == nil {
			continue
		}
		a.AddQuest(questCfg)
	}
	a.Base.ExchangeRecord = nil
	a.SetDirty()
	slog.Debug("Reset", "playerId", a.Activities.GetPlayer().GetId(), "activityId", a.GetId())
}

func (a *ActivityDefault) OnEnd(t time.Time) {
	activityCfg := a.GetActivityCfg()
	if activityCfg.RemoveDataWhenEnd {
		a.Activities.RemoveActivity(a.GetId())
	}
}

// 领取活动任务奖励
func (a *ActivityDefault) ReceiveReward(cfgId int32) {
	activityCfg := a.GetActivityCfg()
	if activityCfg == nil {
		logger.Debug("%v ReceiveRewards activityCfg nil %v", a.Activities.GetPlayer().GetId(), a.GetId())
		return
	}
	progress := a.GetQuest(cfgId)
	if progress == nil {
		logger.Debug("%v ReceiveReward progress nil %v %v", a.Activities.GetPlayer().GetId(), a.GetId(), cfgId)
		return
	}
	if progress.IsReceiveReward {
		logger.Debug("%v ReceiveReward repeat %v %v", a.Activities.GetPlayer().GetId(), a.GetId(), cfgId)
		return
	}
	if !slices.Contains(activityCfg.QuestIds, cfgId) {
		return
	}
	questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(cfgId)
	if questCfg == nil {
		logger.Debug("%v ReceiveReward questCfg nil %v %v", a.Activities.GetPlayer().GetId(), a.GetId(), cfgId)
		return
	}
	if progress.Progress < questCfg.Progress.GetTotal() {
		logger.Debug("%v ReceiveReward progress err %v %v (%v < %v)", a.Activities.GetPlayer().GetId(), a.GetId(), cfgId,
			progress.Progress, questCfg.Progress.GetTotal())
		return
	}
	progress.IsReceiveReward = true
	a.Activities.GetPlayer().GetBags().AddItems(questCfg.GetRewards())
	a.SetDirty()
	// 自动接后续任务
	for _, nextQuestId := range questCfg.GetNextQuests() {
		nextQuestCfg := cfg.GetQuestCfgMgr().GetQuestCfg(nextQuestId)
		if nextQuestCfg == nil {
			continue
		}
		a.AddQuest(nextQuestCfg)
	}
	logger.Debug("%v ReceiveReward %v %v", a.Activities.GetPlayer().GetId(), a.GetId(), cfgId)
}

// 兑换物品
//
//	商店也是一种兑换功能
func (a *ActivityDefault) Exchange(exchangeCfgId int32) {
	activityCfg := a.GetActivityCfg()
	if activityCfg == nil {
		logger.Debug("%v Exchange activityCfg nil %v", a.Activities.GetPlayer().GetId(), a.GetId())
		return
	}
	if !slices.Contains(activityCfg.ExchangeIds, exchangeCfgId) {
		return
	}
	exchangeCfg := cfg.GetTemplateCfgMgr().GetExchangeCfg(exchangeCfgId)
	if exchangeCfg == nil {
		logger.Debug("%v Exchange exchangeCfg nil %v %v", a.Activities.GetPlayer().GetId(), a.GetId(), exchangeCfgId)
		return
	}
	exchangeCount := a.GetExchangeCount(exchangeCfgId)
	if exchangeCount >= exchangeCfg.CountLimit {
		logger.Debug("%v Exchange CountLimit %v %v %v", a.Activities.GetPlayer().GetId(), a.GetId(), exchangeCfgId, exchangeCount)
		return
	}
	// TODO: 检查兑换条件
	if !a.Activities.GetPlayer().GetBags().IsEnough(exchangeCfg.Consumes) {
		logger.Debug("%v Exchange ConsumeItems notEnough %v %v", a.Activities.GetPlayer().GetId(), a.GetId(), exchangeCfgId)
		return
	}
	a.AddExchangeCount(exchangeCfgId, 1)
	a.Activities.GetPlayer().GetBags().DelItems(exchangeCfg.Consumes)
	a.Activities.GetPlayer().GetBags().AddItems(exchangeCfg.Rewards)
}

func (a *ActivityDefault) GetExchangeCount(exchangeCfgId int32) int32 {
	if a.Base.ExchangeRecord == nil {
		return 0
	}
	return a.Base.ExchangeRecord[exchangeCfgId]
}

func (a *ActivityDefault) AddExchangeCount(exchangeCfgId, exchangeCount int32) {
	if a.Base.ExchangeRecord == nil {
		a.Base.ExchangeRecord = make(map[int32]int32)
	}
	a.Base.ExchangeRecord[exchangeCfgId] += exchangeCount
	a.SetDirty()
}

func (a *ActivityDefault) GetPropertyInt32(propertyName string) int32 {
	if property, ok := a.Base.PropertiesInt32[propertyName]; ok {
		return property
	}
	switch propertyName {
	case "DayCount":
		// 当前是参加这个活动的第几天,从1开始
		days := util.DayCount(a.Activities.GetPlayer().GetTimerEntries().Now(), time.Unix(int64(a.Base.JoinTime), 0))
		return int32(days) + 1
	default:
		logger.Error("Not support property %v %v", a.GetId(), propertyName)
	}
	return 0
}
