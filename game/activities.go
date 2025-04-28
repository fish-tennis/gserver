package game

import (
	"fmt"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
	"time"
)

var (
	// 活动模板的构造函数注册表
	_activityTemplateCtorMap = make(map[string]func(activities internal.ActivityMgr, activityCfg *pb.ActivityCfg, args interface{}) internal.Activity)
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
	// eventMapping map[string][]ProgressHolder // key:eventName
}

func (p *Player) GetActivities() *Activities {
	return p.GetComponentByName(ComponentNameActivities).(*Activities)
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

func (a *Activities) GetActivity(activityId int32) internal.Activity {
	activity, _ := a.Data.Data[activityId]
	return activity
}

func (a *Activities) GetActivityDefault(activityId int32) *ActivityDefault {
	activityDefault, _ := a.GetActivity(activityId).(*ActivityDefault)
	return activityDefault
}

func (a *Activities) AddNewActivity(activityCfg *pb.ActivityCfg, t time.Time) internal.Activity {
	activity := CreateNewActivity(activityCfg.CfgId, a, t)
	if activity == nil {
		slog.Error("AddNewActivityErr", "activityId", activityCfg.CfgId)
		return nil
	}
	a.Data.Set(activity.GetId(), activity)
	slog.Debug("AddNewActivity", "playerId", a.GetPlayer().GetId(),
		"activityId", activityCfg.CfgId, "activityName", activityCfg.Name)
	return activity
}

func (a *Activities) RemoveActivity(activityId int32) {
	a.Data.Delete(activityId)
}

func (a *Activities) AddAllActivities(t time.Time) {
	cfg.GetActivityCfgMgr().Range(func(activityCfg *pb.ActivityCfg) bool {
		if a.GetActivity(activityCfg.CfgId) == nil {
			if a.CanJoin(activityCfg, t) {
				a.AddNewActivity(activityCfg, t)
			}
		}
		return true
	})
}

func (a *Activities) LoadFromBytesMap(bytesMap any) error {
	sourceData := bytesMap.(map[int32][]byte)
	for activityId, bytes := range sourceData {
		// 动态构建活动对象
		activity := CreateNewActivity(activityId, a, a.GetPlayer().GetTimerEntries().Now())
		if activity == nil {
			logger.Error(fmt.Sprintf("activity nil id:%v", activityId))
			continue
		}
		err := gentity.LoadObjData(activity, bytes)
		if err != nil {
			logger.Error(fmt.Sprintf("activity load %v err:%v", activityId, err.Error()))
			return err
		}
		a.Data.Data[activityId] = activity // 加载数据,不设置dirty
	}
	return nil
}

func (a *Activities) OnDataLoad() {
	a.Data.Range(func(k int32, activity internal.Activity) bool {
		if dataLoader, ok := activity.(internal.DataLoader); ok {
			dataLoader.OnDataLoad()
			slog.Debug("OnDataLoad", "id", activity.GetId())
		}
		return true
	})
}

func (a *Activities) SyncDataToClient() {
	// 同步各活动的数据给客户端
	a.Data.Range(func(k int32, activity internal.Activity) bool {
		if dataSyncer, ok := activity.(DataSyncer); ok {
			dataSyncer.SyncDataToClient()
			slog.Debug("SyncDataToClient", "id", activity.GetId())
		}
		return true
	})
}

// 事件分发
func (a *Activities) OnEvent(event interface{}) {
	a.Data.Range(func(k int32, activity internal.Activity) bool {
		activity.OnEvent(event)
		return true
	})
}

// 检查活动是否能参加
func (a *Activities) CanJoin(activityCfg *pb.ActivityCfg, t time.Time) bool {
	if activityCfg.IsOff {
		return false
	}
	if activityCfg.MinPlayerLevel > 0 && a.GetPlayer().GetLevel() < activityCfg.MinPlayerLevel {
		return false
	}
	if activityCfg.MaxPlayerLevel > 0 && a.GetPlayer().GetLevel() > activityCfg.MaxPlayerLevel {
		return false
	}
	if !a.CheckJoinTime(activityCfg, t) {
		return false
	}
	return true
}

// 检查活动时间能否参加
func (a *Activities) CheckJoinTime(activityCfg *pb.ActivityCfg, t time.Time) bool {
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
func (a *Activities) CheckEndTime(activityCfg *pb.ActivityCfg, t time.Time) bool {
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
func (a *Activities) CheckEnd(t time.Time) {
	for activityId, activity := range a.Data.Data {
		activityCfg := cfg.GetActivityCfgMgr().GetActivityCfg(activityId)
		if activityCfg == nil {
			continue
		}
		if a.CheckEndTime(activityCfg, t) {
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
func (this *ChildActivity) GetActivityCfg() *pb.ActivityCfg {
	return cfg.GetActivityCfgMgr().GetActivityCfg(this.GetId())
}
