package game

import (
	"github.com/fish-tennis/gserver/cfg"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"time"
)

func init() {
	// 自动注册默认活动模板构造函数
	_activityTemplateCtorMap["default"] = func(activities ActivityMgr, activityCfg *cfg.ActivityCfg, args interface{}) Activity {
		newActivity := &ActivityDefault{
			Base: &pb.ActivityDefaultBaseData{
				InitTime: int32(time.Now().Unix()),
			},
		}
		newActivity.Id = activityCfg.CfgId
		newActivity.Activities = activities.(*Activities)
		newActivity.Reset()
		return newActivity
	}
}

// 默认活动模板,支持常见的简单活动
type ActivityDefault struct {
	ChildActivity
	// 基础数据
	Base *pb.ActivityDefaultBaseData `db:"Base"`
}

func (this *ActivityDefault) getProgress(cfgId int32) *pb.ActivityProgressData {
	if this.Base.Progresses == nil {
		return nil
	}
	return this.Base.Progresses[cfgId]
}

// 添加一个活动进度
func (this *ActivityDefault) addProgress(questCfg *cfg.QuestCfg) *pb.ActivityProgressData {
	if !cfg.GetActivityCfgMgr().GetConditionMgr().CheckConditions(&ActivityConditionArg{
		Activities: this.Activities,
		Activity:   this,
	}, questCfg.Conditions) {
		return nil
	}
	if this.Base.Progresses == nil {
		this.Base.Progresses = make(map[int32]*pb.ActivityProgressData)
	}
	progress := &pb.ActivityProgressData{
		CfgId: questCfg.CfgId,
	}
	cfg.GetActivityCfgMgr().GetProgressMgr().InitProgress(this.Activities.GetPlayer(), questCfg.ProgressCfg, progress)
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
	for _, questCfg := range activityCfg.Quests {
		progress := this.getProgress(questCfg.CfgId)
		if progress == nil {
			progress = this.addProgress(questCfg)
			if progress == nil {
				continue
			}
		}
		// 检查进度更新
		if cfg.GetActivityCfgMgr().GetProgressMgr().CheckProgress(event, questCfg.ProgressCfg, progress) {
			this.SetDirty()
			logger.Debug("Activity %v progress:%v", this.GetId(), progress.GetProgress())
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
	if activityCfg.RefreshType == 1 {
		this.Reset()
	}
}

// 重置数据
func (this *ActivityDefault) Reset() {
	this.Base.Progresses = nil
	activityCfg := this.GetActivityCfg()
	for _, questCfg := range activityCfg.Quests {
		this.addProgress(questCfg)
	}
	this.Base.ExchangeRecord = nil
	this.SetDirty()
	logger.Debug("%v Reset %v", this.Activities.GetPlayer().GetId(), this.GetId())
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
	questCfg := activityCfg.GetQuestCfg(cfgId)
	if questCfg == nil {
		logger.Debug("%v ReceiveReward questCfg nil %v %v", this.Activities.GetPlayer().GetId(), this.GetId(), cfgId)
		return
	}
	if progress.Progress < questCfg.ProgressCfg.GetTotal() {
		logger.Debug("%v ReceiveReward progress err %v %v (%v < %v)", this.Activities.GetPlayer().GetId(), this.GetId(), cfgId,
			progress.Progress, questCfg.ProgressCfg.GetTotal())
		return
	}
	progress.IsReceiveReward = true
	this.Activities.GetPlayer().GetBag().AddItems(questCfg.GetRewards())
	this.SetDirty()
	logger.Debug("%v ReceiveReward %v %v", this.Activities.GetPlayer().GetId(), this.GetId(), cfgId)
}

// 兑换物品
//  商店也是一种兑换功能
func (this *ActivityDefault) Exchange(exchangeCfgId int32) {
	activityCfg := this.GetActivityCfg()
	if activityCfg == nil {
		logger.Debug("%v Exchange activityCfg nil %v", this.Activities.GetPlayer().GetId(), this.GetId())
		return
	}
	exchangeCfg := activityCfg.GetExchangeCfg(exchangeCfgId)
	if exchangeCfg == nil {
		logger.Debug("%v Exchange exchangeCfg nil %v %v", this.Activities.GetPlayer().GetId(), this.GetId(), exchangeCfgId)
		return
	}
	exchangeCount := this.getExchangeCount(exchangeCfgId)
	if exchangeCount >= exchangeCfg.CountLimit {
		logger.Debug("%v Exchange CountLimit nil %v %v %v", this.Activities.GetPlayer().GetId(), this.GetId(), exchangeCfgId, exchangeCount)
		return
	}
	if !this.Activities.GetPlayer().GetBag().IsEnough(exchangeCfg.ConsumeItems) {
		logger.Debug("%v Exchange ConsumeItems notEnough %v %v", this.Activities.GetPlayer().GetId(), this.GetId(), exchangeCfgId)
		return
	}
	this.addExchangeCount(exchangeCfgId, 1)
	this.Activities.GetPlayer().GetBag().DelItems(exchangeCfg.ConsumeItems)
	this.Activities.GetPlayer().GetBag().AddItems(exchangeCfg.Rewards)
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
		y, m, d := time.Now().Date()
		initY, initM, initD := time.Unix(int64(this.Base.InitTime), 0).Date()
		nowDate := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
		initDate := time.Date(initY, initM, initD, 0, 0, 0, 0, time.Local)
		days := nowDate.Sub(initDate) / (time.Hour * 24)
		return int32(days) + 1
	default:
		logger.Error("Not support property %v %v", this.GetId(), propertyName)
	}
	return 0
}
