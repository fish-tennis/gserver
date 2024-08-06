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
	_playerComponentRegister.Register(ComponentNameActivities, 100, func(player *Player, _ any) gentity.Component {
		return &Activities{
			PlayerMapDataComponent: NewPlayerMapDataComponent(player, ComponentNameActivities),
			Data:                   gentity.NewMapData[int32, internal.Activity](),
		}
	})
}

// 活动模块
type Activities struct {
	*PlayerMapDataComponent
	Data *gentity.MapData[int32, internal.Activity] `db:""`
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
	activity, _ := this.Data.Data[activityId]
	return activity
}

func (this *Activities) AddNewActivity(activityCfg *cfg.ActivityCfg, t time.Time) internal.Activity {
	activity := CreateNewActivity(activityCfg.CfgId, this, t)
	if activity == nil {
		logger.Error("activity nil %v", activityCfg.CfgId)
		return nil
	}
	this.Data.Set(activity.GetId(), activity)
	logger.Debug("AddNewActivity playerId:%v activityId:%v", this.GetPlayer().GetId(), activityCfg.CfgId)
	return activity
}

func (this *Activities) RemoveActivity(activityId int32) {
	this.Data.Delete(activityId)
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

func (this *Activities) LoadFromBytesMap(bytesMap any) error {
	sourceData := bytesMap.(map[int32][]byte)
	for activityId, bytes := range sourceData {
		// 动态构建活动对象
		activity := CreateNewActivity(activityId, this, this.GetPlayer().GetTimerEntries().Now())
		if activity == nil {
			logger.Error(fmt.Sprintf("activity nil id:%v", activityId))
			continue
		}
		err := gentity.LoadObjData(activity, bytes)
		if err != nil {
			logger.Error(fmt.Sprintf("activity load %v err:%v", activityId, err.Error()))
			return err
		}
		this.Data.Data[activityId] = activity // 加载数据,不设置dirty
	}
	return nil
}

// 事件分发
func (this *Activities) OnEvent(event interface{}) {
	this.Data.Range(func(k int32, activity internal.Activity) bool {
		activity.OnEvent(event)
		return true
	})
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
		if activityCfg.BeginTime > 0 && now < int64(activityCfg.BeginTime) {
			return false
		}
		if activityCfg.EndTime > 0 && now > int64(activityCfg.EndTime) {
			return false
		}

	case int32(pb.TimeType_TimeType_Date):
		nowDateInt := util.ToDateInt(t)
		if activityCfg.BeginTime > 0 && nowDateInt < activityCfg.BeginTime {
			return false
		}
		if activityCfg.EndTime > 0 || nowDateInt > activityCfg.EndTime {
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
	for activityId, activity := range this.Data.Data {
		activityCfg := cfg.GetActivityCfgMgr().GetActivityCfg(activityId)
		if activityCfg == nil {
			continue
		}
		if this.CheckEndTime(activityCfg, t) {
			activity.OnEnd(t)
		}
	}
}

// 子活动,目前限制子活动只能是单保存字段(SingleField)
type ChildActivity struct {
	internal.BaseActivity
	gentity.MapValueDirtyMark[int32]
	Activities *Activities
}

//// 子活动有数据变化时,玩家活动模块设置脏标记
//func (this *ChildActivity) SetDirty() {
//	//this.BaseDirtyMark.SetDirty()
//	this.Activities.SetDirty(this.GetId(), true)
//}

// 活动配置数据
func (this *ChildActivity) GetActivityCfg() *cfg.ActivityCfg {
	return cfg.GetActivityCfgMgr().GetActivityCfg(this.GetId())
}

type ActivityConditionArg struct {
	Activities *Activities
	Activity   internal.Activity
}
