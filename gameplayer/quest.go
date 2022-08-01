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
	finished *FinishedQuests
	// 当前任务列表
	quests *CurQuests
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
	finished *internal.SliceInt32
	// 排重数组也可以使用map代替
	//finished map[int32]int8
}

func (f *FinishedQuests) DbData() (dbData interface{}, protoMarshal bool) {
	return f.finished, false
}

func (f *FinishedQuests) CacheData() interface{} {
	return f.finished
}

func (f *FinishedQuests) Key() string {
	return "finished"
}

func (f *FinishedQuests) GetCacheKey() string {
	return f.quest.GetCacheKey() + f.Key()
}

//func (f *FinishedQuests) GetMapValue(key string) (value interface{}, exists bool) {
//	value,exists = f.finished[int32(util.Atoi(key))]
//	return
//}

func (f *FinishedQuests) Add(finishedQuestId int32) {
	if f.finished.Contains(finishedQuestId) {
		return
	}
	f.finished.Append(finishedQuestId)
	f.SetDirty()
	logger.Debug("add finished %v", finishedQuestId)
}

var _ internal.MapDirtyMark = (*CurQuests)(nil)

// 当前任务列表
type CurQuests struct {
	internal.BaseMapDirtyMark
	quest  *Quest
	quests map[int32]*pb.QuestData
}

func (c *CurQuests) GetMapValue(key string) (value interface{}, exists bool) {
	value, exists = c.quests[int32(util.Atoi(key))]
	return
}

func (c *CurQuests) DbData() (dbData interface{}, protoMarshal bool) {
	return c.quests, true
}

func (c *CurQuests) CacheData() interface{} {
	return c.quests
}

func (c *CurQuests) Key() string {
	return "quests"
}

func (c *CurQuests) GetCacheKey() string {
	return c.quest.GetCacheKey() + c.Key()
}

func (c *CurQuests) Add(questData *pb.QuestData) {
	c.quests[questData.CfgId] = questData
	c.SetDirty(questData.CfgId, true)
	logger.Debug("add quest:%v", questData)
}

func (c *CurQuests) Remove(questId int32) {
	delete(c.quests, questId)
	c.SetDirty(questId, false)
	logger.Debug("remove quest:%v", questId)
}

func (c *CurQuests) fireEvent(event interface{}) {
	for _,questData := range c.quests {
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
		finished: &FinishedQuests{
		},
		quests: &CurQuests{
		},
	}
	component.finished.quest = component
	component.quests.quest = component
	component.checkData()
	return component
}

// 需要保存数据的子模块
func (this *Quest) SaveableChildren() []internal.ChildSaveable {
	return []internal.ChildSaveable{this.finished, this.quests}
}

func (this *Quest) checkData() {
	if this.finished.finished == nil {
		this.finished.finished = new(internal.SliceInt32)
	}
	if this.quests.quests == nil {
		this.quests.quests = make(map[int32]*pb.QuestData)
	}
}

// 事件接口
func (this *Quest) OnEvent(event interface{}) {
	switch event.(type) {
	case *internal.EventPlayerEntryGame:
		// 测试代码
		if len(this.quests.quests) == 0 {
			for _,questCfg := range cfg.GetQuestCfgMgr().GetQuestCfgs() {
				questData := &pb.QuestData{CfgId: questCfg.CfgId}
				this.quests.Add(questData)
			}
		}
	}
}

// 完成任务,领取任务奖励
func (this *Quest) FinishQuests() {
	for questId,questData := range this.quests.quests {
		questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(questData.GetCfgId())
		if questData.GetProgress() >= questCfg.ConditionCfg.Total {
			this.quests.Remove(questId)
			this.finished.Add(questId)
		}
	}
}
