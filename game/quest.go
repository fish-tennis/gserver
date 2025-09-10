package game

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
	"time"
)

const (
	// 组件名
	ComponentNameQuest = "Quest"
)

// 利用go的init进行组件的自动注册
func init() {
	_playerComponentRegister.Register(ComponentNameQuest, 0, func(player *Player, _ any) gentity.Component {
		return &Quest{
			BasePlayerComponent: BasePlayerComponent{
				player: player,
				name:   ComponentNameQuest,
			},
			Finished: gentity.NewMapData[int32, *pb.FinishedQuestData](),
			Quests:   gentity.NewMapData[int32, *pb.QuestData](),
		}
	})
}

// 任务模块
// 有多个子模块
type Quest struct {
	BasePlayerComponent
	// 保存数据的子模块:已完成的任务
	Finished *gentity.MapData[int32, *pb.FinishedQuestData] `child:""`
	// 保存数据的子模块:当前任务列表
	Quests *gentity.MapData[int32, *pb.QuestData] `child:""`
}

func (p *Player) GetQuest() *Quest {
	return p.GetComponentByName(ComponentNameQuest).(*Quest)
}

func (q *Quest) OnDataLoad() {
	// 把已有任务加入到进度更新映射表中
	q.Quests.Range(func(k int32, questData *pb.QuestData) bool {
		questCfg := cfg.Quests.GetCfg(questData.GetCfgId())
		if questCfg == nil {
			logger.Error("questCfg nil %v", questData.GetCfgId())
			return true
		}
		q.GetPlayer().progressEventMapping.AddProgress(questCfg.Progress, questData)
		return true
	})
}

func (q *Quest) SyncDataToClient() {
	q.GetPlayer().Send(&pb.QuestSync{
		Finished: q.Finished.Data,
		Quests:   q.Quests.Data,
	})
}

func (q *Quest) AddQuest(questData *pb.QuestData) {
	questCfg := cfg.Quests.GetCfg(questData.GetCfgId())
	if questCfg == nil {
		slog.Error("AddQuestErr", "questData", questData)
		return
	}
	q.Quests.Set(questData.CfgId, questData)
	// 初始化进度
	if questCfg.Progress != nil {
		if questCfg.Progress.NeedInit {
			internal.InitProgress(q.GetPlayer(), questData, questCfg.Progress)
		}
		q.GetPlayer().progressEventMapping.AddProgress(questCfg.Progress, questData)
	}
	q.GetPlayer().Send(&pb.QuestUpdate{
		QuestCfgId: questData.GetCfgId(),
		Data:       questData,
	})
	slog.Debug("AddQuest", "questData", questData)
}

func (q *Quest) RemoveQuest(questCfgId int32) {
	q.Quests.Delete(questCfgId)
	q.GetPlayer().Send(&pb.QuestRemoveRes{
		QuestCfgId: questCfgId,
	})
}

// 遍历某个活动关联的当前任务
func (q *Quest) RangeByActivityId(activityId int32, f func(questData *pb.QuestData) bool) {
	q.Quests.Range(func(k int32, questData *pb.QuestData) bool {
		if questData.GetActivityId() == activityId {
			return f(questData)
		}
		return true
	})
}

// 检查任务满足接取条件
func (q *Quest) CanAccept(obj any, questCfg *pb.QuestCfg) bool {
	if questCfg.GetPlayerLevel() > 0 && questCfg.GetPlayerLevel() > q.GetPlayer().GetLevel() {
		return false
	}
	return internal.CheckConditions(obj, questCfg.GetConditions())
}

// 玩家等级更新时,自动接任务
func (q *Quest) WhenPlayerLevelup(level int32) {
	if quests, ok := cfg.QuestsByLevel[level]; ok {
		quests.Range(func(questCfg *pb.QuestCfg) bool {
			// 排除其他模块的子任务
			if questCfg.GetQuestType() != 0 {
				return true
			}
			if !q.CanAccept(q.GetPlayer(), questCfg) {
				return true
			}
			if q.Quests.Contains(questCfg.GetCfgId()) {
				return true
			}
			questData := &pb.QuestData{CfgId: questCfg.GetCfgId()}
			q.AddQuest(questData)
			return true
		})
	}
}

// 事件接口
func (q *Quest) TriggerPlayerEntryGame(event *internal.EventPlayerEntryGame) {
	// 给新玩家添加初始任务
	if len(q.Quests.Data) == 0 && len(q.Finished.Data) == 0 {
		// 1级玩家可接的任务
		q.WhenPlayerLevelup(1)
	}
}

func (q *Quest) OnEvent(event interface{}) {
	switch e := event.(type) {
	case *internal.EventDateChange:
		q.Refresh(e.OldDate, e.CurDate)
		return
	}
}

func (q *Quest) Refresh(oldDate time.Time, curDate time.Time) {
	q.Finished.Range(func(questCfgId int32, v *pb.FinishedQuestData) bool {
		questCfg := cfg.Quests.GetCfg(questCfgId)
		if questCfg == nil {
			return true
		}
		if questCfg.GetRefreshType() == int32(pb.RefreshType_RefreshType_Day) {
			q.Finished.Delete(questCfgId)
			// NOTE:暂时和删除当前任务用同一个消息
			q.GetPlayer().Send(&pb.QuestRemoveRes{
				QuestCfgId: questCfgId,
			})
		}
		return true
	})
	q.Quests.Range(func(questCfgId int32, v *pb.QuestData) bool {
		questCfg := cfg.Quests.GetCfg(questCfgId)
		// 活动的子任务由活动接口去处理
		if questCfg == nil || v.ActivityId > 0 {
			return true
		}
		q.RemoveQuest(questCfgId)
		return true
	})
	// 重新接取日常任务,实际项目可能还涉及到随机等额外逻辑,这里简单演示一下,接取所有的满足接取条件的日常任务
	cfg.QuestsDay.Range(func(questCfg *pb.QuestCfg) bool {
		if !q.CanAccept(q.GetPlayer(), questCfg) {
			return true
		}
		if q.Quests.Contains(questCfg.GetCfgId()) {
			return true
		}
		questData := &pb.QuestData{CfgId: questCfg.GetCfgId()}
		q.AddQuest(questData)
		return true
	})
}

// 完成任务的消息回调
// 这种格式写的函数可以自动注册客户端消息回调
func (q *Quest) OnFinishQuestReq(req *pb.FinishQuestReq) (*pb.FinishQuestRes, error) {
	logger.Debug("OnFinishQuestReq:%v", req)
	res := &pb.FinishQuestRes{}
	for _, questCfgId := range req.GetQuestCfgIds() {
		if questData, ok := q.Quests.Data[questCfgId]; ok {
			questCfg := cfg.Quests.GetCfg(questData.GetCfgId())
			if questData.GetProgress() >= questCfg.Progress.GetTotal() {
				q.Quests.Delete(questData.GetCfgId())
				finishedData := &pb.FinishedQuestData{
					Timestamp: int32(q.GetPlayer().GetTimerEntries().Now().Unix()),
				}
				q.Finished.Set(questData.GetCfgId(), finishedData)
				q.GetPlayer().progressEventMapping.RemoveProgress(questCfg.Progress, questData.GetCfgId())
				// 任务奖励
				q.GetPlayer().GetBags().AddItems(questCfg.GetRewards())
				res.QuestCfgIds = append(res.QuestCfgIds, questCfgId)
				res.FinishedQuestDatas = append(res.FinishedQuestDatas, finishedData)
				// 任务链
				for _, nextQuestId := range questCfg.GetNextQuests() {
					nextQuestCfg := cfg.Quests.GetCfg(nextQuestId)
					if nextQuestCfg == nil {
						logger.Error("nextQuestCfg nil %v", nextQuestId)
						continue
					}
					var checkObj any
					if questData.GetActivityId() > 0 {
						// 活动任务要特殊处理,传入活动对象
						checkObj = q.GetPlayer().GetActivities().GetActivity(questData.GetActivityId())
					}
					if checkObj == nil {
						checkObj = q.GetPlayer()
					}
					if !q.CanAccept(checkObj, nextQuestCfg) {
						continue
					}
					if q.Quests.Contains(questCfg.GetCfgId()) {
						continue
					}
					q.AddQuest(&pb.QuestData{CfgId: nextQuestId, ActivityId: questData.GetActivityId()})
				}
			}
		}
	}
	return res, nil
}
