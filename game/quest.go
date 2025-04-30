package game

import (
	"errors"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
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
		questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(questData.GetCfgId())
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
	questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(questData.GetCfgId())
	if questCfg == nil {
		slog.Error("AddQuestErr", "questData", questData)
		return
	}
	q.Quests.Set(questData.CfgId, questData)
	// 初始化进度
	if questCfg.Progress != nil {
		if questCfg.Progress.NeedInit {
			cfg.GetQuestCfgMgr().GetProgressMgr().InitProgress(questData, q.GetPlayer(), questCfg.Progress)
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

// 事件接口
func (q *Quest) TriggerPlayerEntryGame(event *internal.EventPlayerEntryGame) {
	// 测试代码:给新玩家添加初始任务
	if len(q.Quests.Data) == 0 && len(q.Finished.Data) == 0 {
		cfg.GetQuestCfgMgr().Range(func(questCfg *pb.QuestCfg) bool {
			// 排除其他模块的子任务
			if questCfg.GetQuestType() != 0 {
				return true
			}
			if !cfg.GetQuestCfgMgr().GetConditionMgr().CheckConditions(q.GetPlayer(), questCfg.Conditions) {
				return true
			}
			questData := &pb.QuestData{CfgId: questCfg.CfgId}
			q.AddQuest(questData)
			return true
		})
	}
}

// 完成任务的消息回调
// 这种格式写的函数可以自动注册客户端消息回调
func (q *Quest) OnFinishQuestReq(req *pb.FinishQuestReq) (*pb.FinishQuestRes, error) {
	logger.Debug("OnFinishQuestReq:%v", req)
	if questData, ok := q.Quests.Data[req.QuestCfgId]; ok {
		questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(questData.GetCfgId())
		if questData.GetProgress() >= questCfg.Progress.GetTotal() {
			q.Quests.Delete(questData.GetCfgId())
			q.Finished.Set(questData.GetCfgId(), &pb.FinishedQuestData{
				Timestamp: int32(q.GetPlayer().GetTimerEntries().Now().Unix()),
			})
			q.GetPlayer().progressEventMapping.RemoveProgress(questCfg.Progress, questData.GetCfgId())
			// 任务奖励
			q.GetPlayer().GetBags().AddItems(questCfg.GetRewards())
			return &pb.FinishQuestRes{
				QuestCfgId: questData.GetCfgId(),
			}, nil
		}
		return nil, errors.New("quest not finish")
	}
	return nil, errors.New("quest not exist")
}
