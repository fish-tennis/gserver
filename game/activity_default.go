package game

import (
	"errors"
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
	// 当ActivityDefault无法满足业务需求时,有2种扩展方式
	// 假设需要扩展的活动模板名为PlanB
	// 方案1:
	//     注册一个PlanB的活动构造函数,依然返回ActivityDefault,但是有自己的初始化过程,可以扩展pb.ActivityDefaultBaseData的字段
	//     优点: 不同的活动使用同样的数据结构和接口,这样,策划很容易配置,后台管理系统也很好写,所有活动都使用一套通用的操作
	//          避免了新加一个活动,就要增加新的配置表,后台管理系统就要新写代码的重复工作
	//     实践: 本人在项目中遇到的各种奇奇怪怪的活动需求,暂时还没有ActivityDefault实现不了的,当然注册了很多不同的构造接口,以满足不同
	//          的活动需求,当然实际项目中的ActivityDefault代码更多一些,这个需要自己根据业务需求来调整
	//          本人非常推荐方案1,如果能满足业务需求,则该方案好处多多!
	// 方案2:
	//     注册一个PlanB的活动构造函数,返回Activity的新实现类ActivityPlanB,有自己的结构和接口(工厂模式)
	//     这个是从代码的角度,来满足未知的扩展需求
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
		// TODO:这里可以扩展自定义的初始化过程
		return newActivity
	}
}

// 默认活动模板,支持常见的活动
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
	// 活动的子任务跟玩家的普通任务是同一个模块
	a.Activities.GetPlayer().GetQuest().AddQuest(questData)
}

// 响应事件
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

// 日期变化,进行相应刷新
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
	// TODO:其他刷新方式
}

// 重置数据
func (a *ActivityDefault) Reset() {
	// 活动有可能重开,先删除跟该活动关联的旧任务数据
	a.Activities.GetPlayer().GetQuest().RangeByActivityId(a.GetId(), func(v *pb.QuestData) bool {
		// TODO:判断刷新类型,如每日任务
		a.Activities.GetPlayer().GetQuest().RemoveQuest(v.GetCfgId())
		return true
	})
	activityCfg := a.GetActivityCfg()
	for _, questId := range activityCfg.QuestIds {
		questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(questId)
		if questCfg == nil {
			continue
		}
		// TODO:判断刷新类型,如每日任务
		a.AddQuest(questCfg)
	}
	a.Base.ExchangeRecord = nil
	a.SetDirty()
	slog.Debug("Reset", "playerId", a.Activities.GetPlayer().GetId(),
		"activityId", a.GetId(), "activityName", activityCfg.Name)
}

// 活动结束,进行一些清理工作
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
func (a *ActivityDefault) Exchange(exchangeCfgId, exchangeCount int32) error {
	if exchangeCount <= 0 {
		return errors.New("exchangeCount <= 0")
	}
	activityCfg := a.GetActivityCfg()
	if activityCfg == nil {
		logger.Debug("%v Exchange activityCfg nil %v", a.Activities.GetPlayer().GetId(), a.GetId())
		return errors.New("activityCfg nil")
	}
	if !slices.Contains(activityCfg.ExchangeIds, exchangeCfgId) {
		return errors.New("exchangeCfgId err")
	}
	exchangeCfg := cfg.GetTemplateCfgMgr().GetExchangeCfg(exchangeCfgId)
	if exchangeCfg == nil {
		logger.Debug("%v Exchange exchangeCfg nil %v %v", a.Activities.GetPlayer().GetId(), a.GetId(), exchangeCfgId)
		return errors.New("exchangeCfg nil")
	}
	curExchangeCount := a.GetExchangeCount(exchangeCfgId)
	if exchangeCfg.CountLimit > 0 && curExchangeCount+exchangeCount > exchangeCfg.CountLimit {
		logger.Debug("%v Exchange CountLimit %v %v %v", a.Activities.GetPlayer().GetId(), a.GetId(), exchangeCfgId, exchangeCount)
		return errors.New("exchangeCountLimit")
	}
	// TODO: 检查兑换条件
	// 如果配置了兑换消耗物品,就是购买礼包类的活动,如果不配置,就是免费礼包类的活动
	if !a.Activities.GetPlayer().GetBags().IsEnough(exchangeCfg.Consumes) {
		logger.Debug("%v Exchange ConsumeItems notEnough %v %v", a.Activities.GetPlayer().GetId(), a.GetId(), exchangeCfgId)
		return errors.New("ConsumeItemsNotEnough")
	}
	a.addExchangeCount(exchangeCfgId, exchangeCount)                  // 记录兑换次数
	a.Activities.GetPlayer().GetBags().DelItems(exchangeCfg.Consumes) // 消耗
	a.Activities.GetPlayer().GetBags().AddItems(exchangeCfg.Rewards)  // 购买
	return nil
}

// 获取某个礼包的已兑换次数
func (a *ActivityDefault) GetExchangeCount(exchangeCfgId int32) int32 {
	if a.Base.ExchangeRecord == nil {
		return 0
	}
	return a.Base.ExchangeRecord[exchangeCfgId]
}

func (a *ActivityDefault) addExchangeCount(exchangeCfgId, exchangeCount int32) {
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

// 同步数据给客户端
func (a *ActivityDefault) SyncDataToClient() {
	a.Activities.GetPlayer().Send(&pb.ActivitySync{
		ActivityId: a.GetId(),
		BaseData:   a.Base,
	})
}
