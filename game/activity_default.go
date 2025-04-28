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

// 添加一个活动任务
func (a *ActivityDefault) AddQuest(questCfg *pb.QuestCfg) {
	if !cfg.GetActivityCfgMgr().GetConditionMgr().CheckConditions(a, questCfg.Conditions) {
		return
	}
	questData := &pb.QuestData{
		CfgId:      questCfg.CfgId,
		ActivityId: a.GetId(), // 关联该任务属于哪个活动
	}
	a.Activities.GetPlayer().GetQuest().AddQuest(questData)
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
	// 活动有可能重开,先删除跟该活动关联的旧任务数据
	a.Activities.GetPlayer().GetQuest().RangeByActivityId(a.GetId(), func(v *pb.QuestData) bool {
		a.Activities.GetPlayer().GetQuest().RemoveQuest(v.GetCfgId())
		return true
	})
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
	slog.Debug("Reset", "playerId", a.Activities.GetPlayer().GetId(),
		"activityId", a.GetId(), "activityName", activityCfg.Name)
}

func (a *ActivityDefault) OnEnd(t time.Time) {
	activityCfg := a.GetActivityCfg()
	if activityCfg.RemoveDataWhenEnd {
		a.Activities.RemoveActivity(a.GetId())
		// 删除任务关联的任务
		a.Activities.GetPlayer().GetQuest().RangeByActivityId(a.GetId(), func(questData *pb.QuestData) bool {
			a.Activities.GetPlayer().GetQuest().RemoveQuest(questData.GetCfgId())
			return true
		})
	}
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
