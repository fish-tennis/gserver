package game

import (
	"github.com/fish-tennis/gserver/cfg"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
	"math/rand"
	"time"
)

func init() {
	// 假设有个活动的需求是: 参加活动时,随机一个任务并且每日刷新(刷新一个随机任务覆盖之前的),该活动会配置多个任务供随机
	// 依然使用ActivityDefault实现,但是注册一个新的活动模板名randomQuest,并自定义初始化和刷新接口
	_activityTemplateCtorMap["randomQuest"] = func(activities ActivityMgr, activityCfg *pb.ActivityCfg, _ any) Activity {
		newActivity := newActivityDefault(activities, activityCfg)
		newActivity.customInitFn = randomQuestInit
		newActivity.customRefreshFn = randomQuestRefresh
		return newActivity
	}
}

func randomQuestInit(a *ActivityDefault, t time.Time) {
	activityCfg := a.GetActivityCfg()
	if len(activityCfg.QuestIds) == 0 {
		slog.Error("randomQuestInitErr")
		return
	}
	questId := activityCfg.QuestIds[rand.Intn(len(activityCfg.QuestIds))]
	questCfg := cfg.Quests.GetCfg(questId)
	if questCfg == nil {
		slog.Error("randomQuestInitErr")
		return
	}
	a.AddQuest(questCfg)
	a.SetPropertyInt32("QuestId", questId) // 记录随机出来的任务id
	slog.Debug("randomQuestInit", "pid", a.Activities.GetPlayer().GetId(),
		"activityId", a.GetId(), "activityName", activityCfg.Name, "questId", questId)
}

func randomQuestRefresh(a *ActivityDefault, t time.Time, refreshType int32) {
	// 先删除之前随机出来的任务
	a.Activities.GetPlayer().GetQuest().RemoveQuest(a.GetPropertyInt32("QuestId"))
	// 再重新随机一个
	randomQuestInit(a, t)
	a.defaultRefreshExchange(t, refreshType) // 兑换的刷新继续复用默认接口
	slog.Debug("randomQuestRefresh", "pid", a.Activities.GetPlayer().GetId(), "activityId", a.GetId())
}
