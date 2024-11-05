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

func (this *ActivityDefault) getProgress(cfgId int32) *pb.ActivityProgressData {
	if this.Base.Progresses == nil {
		return nil
	}
	return this.Base.Progresses[cfgId]
}

// 添加一个活动进度
func (this *ActivityDefault) addProgress(questCfg *pb.QuestCfg) *pb.ActivityProgressData {
	if !cfg.GetActivityCfgMgr().GetConditionMgr().CheckConditions(this, questCfg.Conditions) {
		return nil
	}
	if questCfg.Progress == nil {
		return nil
	}
	if this.Base.Progresses == nil {
		this.Base.Progresses = make(map[int32]*pb.ActivityProgressData)
	}
	progress := &pb.ActivityProgressData{
		CfgId: questCfg.CfgId,
	}
	if questCfg.Progress.NeedInit {
		cfg.GetActivityCfgMgr().GetProgressMgr().InitProgress(this, questCfg.Progress, progress)
	}
	this.Base.Progresses[progress.CfgId] = progress
	this.SetDirty()
	return progress
}

func (this *ActivityDefault) OnEvent(event interface{}) {
	activityCfg := this.GetActivityCfg()
	if activityCfg == nil {
		return
	}
	switch e := event.(type) {
	case *EventDateChange:
		this.OnDateChange(e.OldDate, e.CurDate)
		return
	}
	for _, questId := range activityCfg.QuestIds {
		questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(questId)
		if questCfg == nil {
			continue
		}
		progress := this.getProgress(questId)
		if progress == nil {
			progress = this.addProgress(questCfg)
			if progress == nil {
				continue
			}
		}
		// 检查进度更新
		if cfg.GetActivityCfgMgr().GetProgressMgr().CheckProgress(event, questCfg.Progress, progress) {
			this.SetDirty()
			slog.Debug("ActivityProgressUpdate", "id", this.GetId(), "questId", questId, "progress", progress.GetProgress())
		}
	}
}

func (this *ActivityDefault) OnDateChange(oldDate time.Time, curDate time.Time) {
	activityCfg := this.GetActivityCfg()
	if activityCfg == nil {
		logger.Debug("%v OnDateChange activityCfg nil %v", this.Activities.GetPlayer().GetId(), this.GetId())
		return
	}
	// 每日刷新
	if activityCfg.RefreshType == int32(pb.RefreshType_RefreshType_Day) {
		this.Reset()
	}
}

// 重置数据
func (this *ActivityDefault) Reset() {
	this.Base.Progresses = nil
	activityCfg := this.GetActivityCfg()
	for _, questId := range activityCfg.QuestIds {
		questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(questId)
		if questCfg == nil {
			continue
		}
		this.addProgress(questCfg)
	}
	this.Base.ExchangeRecord = nil
	this.SetDirty()
	slog.Debug("Reset", "playerId", this.Activities.GetPlayer().GetId(), "activityId", this.GetId())
}

func (this *ActivityDefault) OnEnd(t time.Time) {
	activityCfg := this.GetActivityCfg()
	if activityCfg.RemoveDataWhenEnd {
		this.Activities.RemoveActivity(this.GetId())
	}
}

// 领取活动任务奖励
func (this *ActivityDefault) ReceiveReward(cfgId int32) {
	activityCfg := this.GetActivityCfg()
	if activityCfg == nil {
		logger.Debug("%v ReceiveRewards activityCfg nil %v", this.Activities.GetPlayer().GetId(), this.GetId())
		return
	}
	progress := this.getProgress(cfgId)
	if progress == nil {
		logger.Debug("%v ReceiveReward progress nil %v %v", this.Activities.GetPlayer().GetId(), this.GetId(), cfgId)
		return
	}
	if progress.IsReceiveReward {
		logger.Debug("%v ReceiveReward repeat %v %v", this.Activities.GetPlayer().GetId(), this.GetId(), cfgId)
		return
	}
	if !slices.Contains(activityCfg.QuestIds, cfgId) {
		return
	}
	questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(cfgId)
	if questCfg == nil {
		logger.Debug("%v ReceiveReward questCfg nil %v %v", this.Activities.GetPlayer().GetId(), this.GetId(), cfgId)
		return
	}
	if progress.Progress < questCfg.Progress.GetTotal() {
		logger.Debug("%v ReceiveReward progress err %v %v (%v < %v)", this.Activities.GetPlayer().GetId(), this.GetId(), cfgId,
			progress.Progress, questCfg.Progress.GetTotal())
		return
	}
	progress.IsReceiveReward = true
	this.Activities.GetPlayer().GetBags().AddItems(questCfg.GetRewards())
	this.SetDirty()
	// 自动接后续任务
	for _, nextQuestId := range questCfg.GetNextQuests() {
		nextQuestCfg := cfg.GetQuestCfgMgr().GetQuestCfg(nextQuestId)
		if nextQuestCfg == nil {
			continue
		}
		this.addProgress(nextQuestCfg)
	}
	logger.Debug("%v ReceiveReward %v %v", this.Activities.GetPlayer().GetId(), this.GetId(), cfgId)
}

// 兑换物品
//
//	商店也是一种兑换功能
func (this *ActivityDefault) Exchange(exchangeCfgId int32) {
	activityCfg := this.GetActivityCfg()
	if activityCfg == nil {
		logger.Debug("%v Exchange activityCfg nil %v", this.Activities.GetPlayer().GetId(), this.GetId())
		return
	}
	if !slices.Contains(activityCfg.ExchangeIds, exchangeCfgId) {
		return
	}
	exchangeCfg := cfg.GetTemplateCfgMgr().GetExchangeCfg(exchangeCfgId)
	if exchangeCfg == nil {
		logger.Debug("%v Exchange exchangeCfg nil %v %v", this.Activities.GetPlayer().GetId(), this.GetId(), exchangeCfgId)
		return
	}
	exchangeCount := this.getExchangeCount(exchangeCfgId)
	if exchangeCount >= exchangeCfg.CountLimit {
		logger.Debug("%v Exchange CountLimit %v %v %v", this.Activities.GetPlayer().GetId(), this.GetId(), exchangeCfgId, exchangeCount)
		return
	}
	// TODO: 检查兑换条件
	if !this.Activities.GetPlayer().GetBags().IsEnough(exchangeCfg.Consumes) {
		logger.Debug("%v Exchange ConsumeItems notEnough %v %v", this.Activities.GetPlayer().GetId(), this.GetId(), exchangeCfgId)
		return
	}
	this.addExchangeCount(exchangeCfgId, 1)
	this.Activities.GetPlayer().GetBags().DelItems(exchangeCfg.Consumes)
	this.Activities.GetPlayer().GetBags().AddItems(exchangeCfg.Rewards)
}

func (this *ActivityDefault) getExchangeCount(exchangeCfgId int32) int32 {
	if this.Base.ExchangeRecord == nil {
		return 0
	}
	return this.Base.ExchangeRecord[exchangeCfgId]
}

func (this *ActivityDefault) addExchangeCount(exchangeCfgId, exchangeCount int32) {
	if this.Base.ExchangeRecord == nil {
		this.Base.ExchangeRecord = make(map[int32]int32)
	}
	this.Base.ExchangeRecord[exchangeCfgId] += exchangeCount
	this.SetDirty()
}

func (this *ActivityDefault) GetPropertyInt32(propertyName string) int32 {
	switch propertyName {
	case "DayCount":
		// 当前是参加这个活动的第几天,从1开始
		days := util.DayCount(this.Activities.GetPlayer().GetTimerEntries().Now(), time.Unix(int64(this.Base.JoinTime), 0))
		return int32(days) + 1
	default:
		logger.Error("Not support property %v %v", this.GetId(), propertyName)
	}
	return 0
}
