package game

import (
	"fmt"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/cfg"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
)

var (
	// 活动模板的构造函数注册表
	_activityTemplateCtorMap = make(map[string]func(activities ActivityMgr, activityCfg *cfg.ActivityCfg, args interface{}) Activity)
)

// 活动模块
type Activities struct {
	PlayerMapDataComponent
	Data map[int32]Activity `db:"Data"`
}

func NewActivities(player *Player) *Activities {
	component := &Activities{
		PlayerMapDataComponent: *NewPlayerMapDataComponent(player, "Activities"),
		Data:                   make(map[int32]Activity),
	}
	return component
}

// 根据模板创建活动对象
func CreateNewActivity(activityCfgId int32, activities ActivityMgr) Activity {
	activityCfg := cfg.GetActivityCfgMgr().GetActivityCfg(activityCfgId)
	if activityCfg == nil {
		logger.Error("activityCfg nil %v", activityCfgId)
		return nil
	}
	if activityCtor,ok := _activityTemplateCtorMap[activityCfg.GetTemplate()]; ok {
		return activityCtor(activities, activityCfg, nil)
	}
	logger.Error("activityCfg nil %v", activityCfgId)
	return nil
}

func (this *Activities) GetActivity(activityId int32) Activity {
	activity,_ := this.Data[activityId]
	return activity
}

func (this *Activities) AddNewActivity(activityCfgId int32) Activity {
	activity := CreateNewActivity(activityCfgId, this)
	if activity == nil {
		logger.Error("activity nil %v", activityCfgId)
		return nil
	}
	this.Data[activityCfgId] = activity
	this.SetDirty(activityCfgId, true)
	logger.Debug("AddNewActivity playerId:%v activityId:%v", this.GetPlayer().GetId(), activityCfgId)
	return activity
}

func (this *Activities) LoadData(sourceData map[int32][]byte) {
	for activityId,bytes := range sourceData {
		// 动态构建活动对象
		activity := CreateNewActivity(activityId, this)
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

func (this *Activities) OnEvent(event interface{}) {
	for _,activity := range this.Data {
		activity.OnEvent(event)
	}
}

// 子活动
type ChildActivity struct {
	gentity.BaseDirtyMark
	BaseActivity
	Activities *Activities
}

func (this *ChildActivity) SetDirty() {
	this.BaseDirtyMark.SetDirty()
	this.Activities.SetDirty(this.GetId(), false)
}
