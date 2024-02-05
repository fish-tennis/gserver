package game

import (
	"github.com/fish-tennis/gserver/cfg"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

func init() {
	_activityTemplateCtorMap["default"] = func(activities ActivityMgr, activityCfg *cfg.ActivityCfg, args interface{}) Activity {
		newActivity := &ActivityDefault{
			Progresses: make(map[int32]*pb.QuestData),
		}
		newActivity.Id = activityCfg.CfgId
		newActivity.Activities = activities.(*Activities)
		for _,questCfg := range activityCfg.Quests {
			newActivity.addProgress(questCfg)
		}
		return newActivity
	}
}

// 默认活动模板,支持常见的简单活动
type ActivityDefault struct {
	ChildActivity
	// 各种进度类的活动 可以用任务来实现
	Progresses map[int32]*pb.QuestData `db:""`
}

func (this *ActivityDefault) addProgress(questCfg *cfg.QuestCfg) *pb.QuestData {
	progress := &pb.QuestData{
		CfgId: questCfg.CfgId,
	}
	cfg.GetActivityCfgMgr().GetConditionMgr().InitCondition(this.Activities.GetPlayer(), questCfg.ConditionCfg, progress)
	this.Progresses[progress.CfgId] = progress
	return progress
}

func (this *ActivityDefault) OnEvent(event interface{}) {
	activityCfg := cfg.GetActivityCfgMgr().GetActivityCfg(this.GetId())
	if activityCfg == nil {
		return
	}
	for _,questCfg := range activityCfg.Quests {
		progress,_ := this.Progresses[questCfg.CfgId]
		if progress == nil {
			progress = this.addProgress(questCfg)
		}
		if cfg.GetActivityCfgMgr().GetConditionMgr().CheckEvent(event, questCfg.ConditionCfg, progress) {
			this.SetDirty()
			logger.Debug("Activity %v progress:%v", this.GetId(), progress.GetProgress())
		}
	}
}
