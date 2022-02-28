package player

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"math/rand"
)

var _ internal.CompositeSaveable = (*Quest)(nil)

// 任务模块
// 演示了一种与Bag不同的组合模块方式
// 与Bag不同,Quest由一个Component和多个ChildSaveable组合而成
// 不同的ChildSaveable可以有不同的数据保存方式
type Quest struct {
	MapDataComponent
	finished *FinishedQuests
	quests *CurQuests
}

var _ internal.DirtyMark = (*FinishedQuests)(nil)

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
	return f.finished,false
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
	//f.finished[finishedQuestId] = 1
	//f.SetDirty(strconv.Itoa(int(finishedQuestId)), true)
}

var _ internal.MapDirtyMark = (*CurQuests)(nil)

type CurQuests struct {
	internal.BaseMapDirtyMark
	quest *Quest
	quests map[int32]*pb.QuestData
}

func (c *CurQuests) GetMapValue(key string) (value interface{}, exists bool) {
	value,exists = c.quests[int32(util.Atoi(key))]
	return
}

func (c *CurQuests) DbData() (dbData interface{}, protoMarshal bool) {
	return c.quests,true
}

func (c *CurQuests) CacheData() interface{} {
	return c.quests
}

func (c *CurQuests) Key() string {
	return "quests"
}

func (c *CurQuests) GetCacheKey() string {
	return c.quest.GetCacheKey() +c.Key()
}

func (c *CurQuests) Add(questData *pb.QuestData) {
	c.quests[questData.CfgId] = questData
	c.SetDirty(questData.CfgId, true)
	logger.Debug("add quest:%v", questData)
}

func NewQuest(player *Player) *Quest {
	component := &Quest{
		MapDataComponent: MapDataComponent{
			BaseComponent: BaseComponent{
				Player: player,
				Name:   "Quest",
			},
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
		//this.finished.finished = make(map[int32]int8)
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
		//this.finished.Add(int32(rand.Intn(100)))
		//logger.Debug("finished:%v", this.finished.finished)
		questData := &pb.QuestData{CfgId: int32(rand.Intn(1000)), Progress: rand.Int31n(100)}
		this.quests.Add(questData)
	}
}