package game

import (
	"errors"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
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
			Finished: new(gentity.SliceData[int32]),
			Quests:   gentity.NewMapData[int32, *pb.QuestData](),
		}
	})
}

// 任务模块
// 有多个子模块
type Quest struct {
	BasePlayerComponent
	// 保存数据的子模块:已完成的任务
	// 保存数据的子模块必须是导出字段(字段名大写开头)
	Finished *gentity.SliceData[int32] `child:"plain"` // 明文保存
	// 保存数据的子模块:当前任务列表
	Quests *gentity.MapData[int32, *pb.QuestData] `child:""`
}

func (this *Player) GetQuest() *Quest {
	return this.GetComponentByName(ComponentNameQuest).(*Quest)
}

func (this *Quest) AddQuest(questData *pb.QuestData) {
	questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(questData.GetCfgId())
	if questCfg == nil {
		logger.Error("questCfg nil %v", questData.GetCfgId())
		return
	}
	this.Quests.Set(questData.CfgId, questData)
	// 初始化进度
	if questCfg.ProgressCfg != nil {
		cfg.GetQuestCfgMgr().GetProgressMgr().InitProgress(this.GetPlayer(), questCfg.ProgressCfg, questData)
	}
	logger.Debug("AddQuest:%v", questData)
}

// 事件接口
func (this *Quest) TriggerPlayerEntryGame(event *internal.EventPlayerEntryGame) {
	// 测试代码:给新玩家添加初始任务
	if len(this.Quests.Data) == 0 && len(this.Finished.Data) == 0 {
		cfg.GetQuestCfgMgr().Range(func(questCfg *cfg.QuestCfg) bool {
			if !cfg.GetQuestCfgMgr().GetConditionMgr().CheckConditions(this.GetPlayer(), questCfg.Conditions) {
				return false
			}
			questData := &pb.QuestData{CfgId: questCfg.CfgId}
			this.AddQuest(questData)
			return true
		})
	}
}

func (this *Quest) OnEvent(event interface{}) {
	for _, questData := range this.Quests.Data {
		questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(questData.GetCfgId())
		if cfg.GetQuestCfgMgr().GetProgressMgr().CheckProgress(event, questCfg.ProgressCfg, questData) {
			this.Quests.SetDirty(questData.GetCfgId(), true)
			logger.Debug("quest %v progress:%v", questData.GetCfgId(), questData.GetProgress())
		}
	}
}

// 完成任务的消息回调
// 这种格式写的函数可以自动注册客户端消息回调
func (this *Quest) OnFinishQuestReq(req *pb.FinishQuestReq) (*pb.FinishQuestRes, error) {
	logger.Debug("OnFinishQuestReq:%v", req)
	if questData, ok := this.Quests.Data[req.QuestCfgId]; ok {
		questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(questData.GetCfgId())
		if questData.GetProgress() >= questCfg.ProgressCfg.GetTotal() {
			this.Quests.Delete(questData.GetCfgId())
			this.Finished.Add(questData.GetCfgId())
			// 任务奖励
			for _, idNum := range questCfg.GetRewards() {
				this.GetPlayer().GetBag().AddItem(idNum.GetCfgId(), idNum.GetNum())
			}
			return &pb.FinishQuestRes{
				QuestCfgId: questData.GetCfgId(),
			}, nil
		}
		return nil, errors.New("quest not finish")
	}
	return nil, errors.New("quest not exist")
}
