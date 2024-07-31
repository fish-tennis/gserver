package game

import (
	"fmt"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"time"
)

var (
	// 活动模板的构造函数注册表
	_activityTemplateCtorMap = make(map[string]func(activities internal.ActivityMgr, activityCfg *cfg.ActivityCfg, args interface{}) internal.Activity)
)

const (
	// 组件名
	ComponentNameActivities = "Activities"
)

// 利用go的init进行组件的自动注册
func init() {
	RegisterPlayerComponentCtor(ComponentNameActivities, 100, func(player *Player, playerData *pb.PlayerData) gentity.Component {
		component := &Activities{
			PlayerMapDataComponent: *NewPlayerMapDataComponent(player, ComponentNameActivities),
			Data:                   make(map[int32]internal.Activity),
		}
		// 这里提前加入组件,因为后面的component.LoadData里,子活动可能需要用到player.GetActivities()
		player.AddComponent(component)
		// 活动组件使用了动态结构,不能使用gentity.LoadData来自动加载数据
		// 自己解析出具体的子活动数据
		component.LoadData(playerData.GetActivities())
		return component
	})
}

// 活动模块
type Activities struct {
	PlayerMapDataComponent
	Data map[int32]internal.Activity `db:"Data"`
}

func (this *Player) GetActivities() *Activities {
	return this.GetComponentByName(ComponentNameActivities).(*Activities)
}

// 根据模板创建活动对象
func CreateNewActivity(activityCfgId int32, activities internal.ActivityMgr, t time.Time) internal.Activity {
	activityCfg := cfg.GetActivityCfgMgr().GetActivityCfg(activityCfgId)
	if activityCfg == nil {
		logger.Error("activityCfg nil %v", activityCfgId)
		return nil
	}
	if activityCtor, ok := _activityTemplateCtorMap[activityCfg.GetTemplate()]; ok {
		return activityCtor(activities, activityCfg, t)
	}
	logger.Error("activityCfg nil %v", activityCfgId)
	return nil
}

func (this *Activities) GetActivity(activityId int32) internal.Activity {
	activity, _ := this.Data[activityId]
	return activity
}

func (this *Activities) AddNewActivity(activityCfg *cfg.ActivityCfg, t time.Time) internal.Activity {
	activity := CreateNewActivity(activityCfg.CfgId, this, t)
	if activity == nil {
		logger.Error("activity nil %v", activityCfg.CfgId)
		return nil
	}
	this.Data[activityCfg.CfgId] = activity
	this.SetDirty(activityCfg.CfgId, true)
	logger.Debug("AddNewActivity playerId:%v activityId:%v", this.GetPlayer().GetId(), activityCfg.CfgId)
	return activity
}

func (this *Activities) RemoveActivity(activityId int32) {
	delete(this.Data, activityId)
	this.SetDirty(activityId, false)
}

func (this *Activities) AddAllActivities(t time.Time) {
	cfg.GetActivityCfgMgr().Range(func(activityCfg *cfg.ActivityCfg) bool {
		if this.GetActivity(activityCfg.CfgId) == nil {
			if this.CanJoin(activityCfg, t) {
				this.AddNewActivity(activityCfg, t)
			}
		}
		return true
	})
}

func (this *Activities) LoadData(sourceData map[int32][]byte) {
	for activityId, bytes := range sourceData {
		// 动态构建活动对象
		activity := CreateNewActivity(activityId, this, this.GetPlayer().GetTimerEntries().Now())
		if activity == nil {
			logger.Error(fmt.Sprintf("activity nil id:%v", activityId))
			continue
		}
		err := gentity.LoadData(activity, bytes)
		if err != nil {
			logger.Error(fmt.Sprintf("activity load %v err:%v", activityId, err.Error()))
			continue
		}
		this.Data[activityId] = activity
	}
}

// 事件分发
func (this *Activities) OnEvent(event interface{}) {
	for _, activity := range this.Data {
		activity.OnEvent(event)
	}
}

// 检查活动是否能参加
func (this *Activities) CanJoin(activityCfg *cfg.ActivityCfg, t time.Time) bool {
	if activityCfg.IsOff {
		return false
	}
	if activityCfg.MinPlayerLevel > 0 && this.GetPlayer().GetLevel() < activityCfg.MinPlayerLevel {
		return false
	}
	if activityCfg.MaxPlayerLevel > 0 && this.GetPlayer().GetLevel() > activityCfg.MaxPlayerLevel {
		return false
	}
	if !this.CheckJoinTime(activityCfg, t) {
		return false
	}
	return true
}

// 检查活动时间能否参加
func (this *Activities) CheckJoinTime(activityCfg *cfg.ActivityCfg, t time.Time) bool {
	switch activityCfg.TimeType {
	case int32(pb.TimeType_TimeType_Timestamp):
		now := t.Unix()
		if now < int64(activityCfg.BeginTime) || now > int64(activityCfg.EndTime) {
			return false
		}

	case int32(pb.TimeType_TimeType_Date):
		nowDateInt := util.ToDateInt(t)
		if nowDateInt < activityCfg.BeginTime || nowDateInt > activityCfg.EndTime {
			return false
		}
	}
	return true
}

// 检查活动时间是否结束
func (this *Activities) CheckEndTime(activityCfg *cfg.ActivityCfg, t time.Time) bool {
	switch activityCfg.TimeType {
	case int32(pb.TimeType_TimeType_Timestamp):
		now := t.Unix()
		if now > int64(activityCfg.EndTime) {
			return true
		}

	case int32(pb.TimeType_TimeType_Date):
		nowDateInt := util.ToDateInt(t)
		if nowDateInt > activityCfg.EndTime {
			return true
		}
	}
	return false
}

// 检查已经结束的活动
func (this *Activities) CheckEnd(t time.Time) {
	for activityId, activity := range this.Data {
		activityCfg := cfg.GetActivityCfgMgr().GetActivityCfg(activityId)
		if activityCfg == nil {
			continue
		}
		if this.CheckEndTime(activityCfg, t) {
			activity.OnEnd(t)
		}
	}
}

// 子活动
type ChildActivity struct {
	gentity.BaseDirtyMark
	internal.BaseActivity
	Activities *Activities
}

// 子活动设置脏标记时,玩家活动模块也设置脏标记
func (this *ChildActivity) SetDirty() {
	this.BaseDirtyMark.SetDirty()
	this.Activities.SetDirty(this.GetId(), true)
}

// 活动配置数据
func (this *ChildActivity) GetActivityCfg() *cfg.ActivityCfg {
	return cfg.GetActivityCfgMgr().GetActivityCfg(this.GetId())
}

type ActivityConditionArg struct {
	Activities *Activities
	Activity   internal.Activity
}
