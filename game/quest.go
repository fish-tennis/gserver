package game

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/gen"
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
	RegisterPlayerComponentCtor(ComponentNameQuest, 0, func(player *Player, playerData *pb.PlayerData) gentity.Component {
		component := &Quest{
			BasePlayerComponent: BasePlayerComponent{
				player: player,
				name:   ComponentNameQuest,
			},
			Finished: &FinishedQuests{},
			Quests:   &CurQuests{},
		}
		component.Finished.quest = component
		component.Quests.quest = component
		component.checkData()
		gentity.LoadData(component, playerData.GetQuest())
		return component
	})
}

// 任务模块
// 有多个子模块
type Quest struct {
	BasePlayerComponent
	// 保存数据的子模块:已完成的任务
	// 保存数据的子模块必须是导出字段(字段名大写开头)
	Finished *FinishedQuests `child:""`
	// 保存数据的子模块:当前任务列表
	Quests *CurQuests `child:""`
}

func (this *Player) GetQuest() *Quest {
	return this.GetComponentByName(ComponentNameQuest).(*Quest)
}

// 已完成的任务
type FinishedQuests struct {
	gentity.BaseDirtyMark
	quest *Quest
	// struct tag里面没有设置保存字段名,会默认使用字段名的全小写形式
	Finished []int32 `db:"plain"` // 基础类型,设置明文存储
}

func (f *FinishedQuests) Add(finishedQuestId int32) {
	if util.ContainsInt32(f.Finished, finishedQuestId) {
		return
	}
	f.Finished = append(f.Finished, finishedQuestId)
	f.SetDirty()
	logger.Debug("add Finished %v", finishedQuestId)
}

// 当前任务列表
type CurQuests struct {
	gentity.BaseMapDirtyMark
	quest *Quest
	// struct tag里面没有设置保存字段名,会默认使用字段名的全小写形式
	Quests map[int32]*pb.QuestData `db:""`
}

func (c *CurQuests) Add(questData *pb.QuestData) {
	questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(questData.GetCfgId())
	if questCfg == nil {
		logger.Error("questCfg nil %v", questData.GetCfgId())
		return
	}
	c.Quests[questData.CfgId] = questData
	c.SetDirty(questData.CfgId, true)
	// 初始化进度
	if questCfg.ProgressCfg != nil {
		cfg.GetQuestCfgMgr().GetProgressMgr().InitProgress(c.quest.GetPlayer(), questCfg.ProgressCfg, questData)
	}
	logger.Debug("add quest:%v", questData)
}

func (c *CurQuests) Remove(questId int32) {
	delete(c.Quests, questId)
	c.SetDirty(questId, false)
	logger.Debug("remove quest:%v", questId)
}

// 触发了事件,检查任务进度的更新
func (c *CurQuests) OnEvent(event interface{}) {
	for _, questData := range c.Quests {
		questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(questData.GetCfgId())
		if cfg.GetQuestCfgMgr().GetProgressMgr().CheckProgress(event, questCfg.ProgressCfg, questData) {
			c.SetDirty(questData.GetCfgId(), true)
			logger.Debug("quest %v progress:%v", questData.GetCfgId(), questData.GetProgress())
		}
	}
}

func (this *Quest) checkData() {
	if this.Finished.Finished == nil {
		this.Finished.Finished = make([]int32, 0)
	}
	if this.Quests.Quests == nil {
		this.Quests.Quests = make(map[int32]*pb.QuestData)
	}
}

// 事件接口
func (this *Quest) OnEvent(event interface{}) {
	switch event.(type) {
	case *internal.EventPlayerEntryGame:
		// 测试代码:给新玩家添加初始任务
		if len(this.Quests.Quests) == 0 && len(this.Finished.Finished) == 0 {
			cfg.GetQuestCfgMgr().Range(func(questCfg *cfg.QuestCfg) bool {
				if !cfg.GetQuestCfgMgr().GetConditionMgr().CheckConditions(this.GetPlayer(), questCfg.Conditions) {
					return false
				}
				questData := &pb.QuestData{CfgId: questCfg.CfgId}
				this.Quests.Add(questData)
				return true
			})
		}
	}
	this.Quests.OnEvent(event)
}

// 完成任务的消息回调
// 这种格式写的函数可以自动注册客户端消息回调
func (this *Quest) OnFinishQuestReq(reqCmd gnet.PacketCommand, req *pb.FinishQuestReq) {
	logger.Debug("OnFinishQuestReq:%v", req)
	if questData, ok := this.Quests.Quests[req.QuestCfgId]; ok {
		questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(questData.GetCfgId())
		if questData.GetProgress() >= questCfg.ProgressCfg.GetTotal() {
			this.Quests.Remove(questData.GetCfgId())
			this.Finished.Add(questData.GetCfgId())
			// 任务奖励
			for _, idNum := range questCfg.GetRewards() {
				this.GetPlayer().GetBag().AddItem(idNum.GetCfgId(), idNum.GetNum())
			}
			gen.SendFinishQuestRes(this.GetPlayer(), &pb.FinishQuestRes{
				QuestCfgId: questData.GetCfgId(),
			})
			return
		}
		this.GetPlayer().SendErrorRes(reqCmd, "quest not finish")
		return
	}
	this.GetPlayer().SendErrorRes(reqCmd, "quest not exist")
}
