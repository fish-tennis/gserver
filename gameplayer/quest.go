package gameplayer

import (
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
)

var _ internal.CompositeSaveable = (*Quest)(nil)

// 任务模块
// 演示了一种与Bag不同的组合模块方式
// 与Bag不同,Quest由一个Component和多个ChildSaveable组合而成
// 不同的ChildSaveable可以有不同的数据保存方式
type Quest struct {
	BasePlayerComponent
	// 已完成的任务
	Finished *FinishedQuests `child:"finished"`
	// 当前任务列表
	Quests *CurQuests `child:"quests"`
}

var _ internal.DirtyMark = (*FinishedQuests)(nil)

// 已完成的任务
type FinishedQuests struct {
	internal.BaseDirtyMark
	quest *Quest
	// 保存数组数据,不能直接使用[]int32
	// SliceInt32只能和DirtyMark配合使用,不支持MapDirtyMark
	// NOTE:使用SliceInt32作用有限,还不如用proto更方便,这里只是演示一种保存方式,实际应用的时候不推荐
	// 暂时也没有实现SliceInt64,SliceProto之类的扩展类,因为redis里不好存储
	Finished []int32 `db:"finished;plain"`
	// 排重数组也可以使用map代替
	//Finished map[int32]int8
}

func (f *FinishedQuests) DbData() (dbData interface{}, protoMarshal bool) {
	return f.Finished, false
}

func (f *FinishedQuests) CacheData() interface{} {
	return f.Finished
}

func (f *FinishedQuests) Key() string {
	return "finished"
}

func (f *FinishedQuests) GetCacheKey() string {
	return f.quest.GetCacheKey() + f.Key()
}

//func (f *FinishedQuests) GetMapValue(key string) (value interface{}, exists bool) {
//	value,exists = f.Finished[int32(util.Atoi(key))]
//	return
//}

func (f *FinishedQuests) Add(finishedQuestId int32) {
	if util.ContainsInt32(f.Finished, finishedQuestId) {
		return
	}
	f.Finished = append(f.Finished, finishedQuestId)
	f.SetDirty()
	logger.Debug("add Finished %v", finishedQuestId)
}

var _ internal.MapDirtyMark = (*CurQuests)(nil)

// 当前任务列表
type CurQuests struct {
	internal.BaseMapDirtyMark
	quest  *Quest
	Quests map[int32]*pb.QuestData `db:"quests"`
}

func (c *CurQuests) GetMapValue(key string) (value interface{}, exists bool) {
	value, exists = c.Quests[int32(util.Atoi(key))]
	return
}

func (c *CurQuests) DbData() (dbData interface{}, protoMarshal bool) {
	return c.Quests, true
}

func (c *CurQuests) CacheData() interface{} {
	return c.Quests
}

func (c *CurQuests) Key() string {
	return "quests"
}

func (c *CurQuests) GetCacheKey() string {
	return c.quest.GetCacheKey() + c.Key()
}

func (c *CurQuests) Add(questData *pb.QuestData) {
	c.Quests[questData.CfgId] = questData
	c.SetDirty(questData.CfgId, true)
	logger.Debug("add quest:%v", questData)
}

func (c *CurQuests) Remove(questId int32) {
	delete(c.Quests, questId)
	c.SetDirty(questId, false)
	logger.Debug("remove quest:%v", questId)
}

func (c *CurQuests) fireEvent(event interface{}) {
	for _,questData := range c.Quests {
		questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(questData.GetCfgId())
		if cfg.GetQuestCfgMgr().GetConditionMgr().CheckEvent(event, questCfg.ConditionCfg, questData) {
			c.SetDirty(questData.GetCfgId(), true)
			logger.Debug("quest %v progress:%v", questData.GetCfgId(), questData.GetProgress())
		}
	}
}

func NewQuest(player *Player) *Quest {
	component := &Quest{
		BasePlayerComponent: BasePlayerComponent{
			player: player,
			name:   "Quest",
		},
		Finished: &FinishedQuests{
		},
		Quests: &CurQuests{
		},
	}
	component.Finished.quest = component
	component.Quests.quest = component
	component.checkData()
	return component
}

// 需要保存数据的子模块
func (this *Quest) SaveableChildren() []internal.ChildSaveable {
	return []internal.ChildSaveable{this.Finished, this.Quests}
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
		// 测试代码
		if len(this.Quests.Quests) == 0 {
			for _,questCfg := range cfg.GetQuestCfgMgr().GetQuestCfgs() {
				questData := &pb.QuestData{CfgId: questCfg.CfgId}
				this.Quests.Add(questData)
			}
		}
	}
}

// 完成任务,领取任务奖励
func (this *Quest) FinishQuests() {
	for questId,questData := range this.Quests.Quests {
		questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(questData.GetCfgId())
		if questData.GetProgress() >= questCfg.ConditionCfg.Total {
			this.Quests.Remove(questId)
			this.Finished.Add(questId)
		}
	}
}
