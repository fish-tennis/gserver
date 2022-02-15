package player

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
)

var _ internal.Saveable = (*Quest)(nil)

// 任务模块
type Quest struct {
	MapDataComponent
	data *pb.Quest
}

func NewQuest(player *Player) *Quest {
	component := &Quest{
		MapDataComponent: MapDataComponent{
			BaseComponent: BaseComponent{
				Player: player,
				Name:   "Quest",
			},
		},
		data: &pb.Quest{
		},
	}
	component.checkData()
	return component
}

func (this *Quest) DbData() (dbData interface{}, protoMarshal bool) {
	// 演示明文保存数据库
	// 优点:便于查看,数据库语言可直接操作字段
	// 缺点:字段名也会保存到数据库,占用空间多
	return this.data,false
}

func (this *Quest) CacheData() interface{} {
	return this.data
}

//
//// 需要保存的数据
//func (this *Quest) Save(forCache bool) (saveData interface{}, saveOption internal.SaveOption) {
//	if forCache {
//		// 保存到缓存时,进行序列化
//		mapData := make(map[string]interface{})
//		mapData["finished"] = this.data.Finished
//		mapData["quests"] = this.data.Quests
//		return mapData,internal.ProtoMarshalMap
//	}
//	//if len(this.dirtyMap) == 0 {
//	//	return nil,true
//	//}
//	//if _,ok := this.dirtyMap["Finished"]; ok {
//	//	db.GetPlayerDb().SaveComponentField(this.GetPlayerId(), this.GetNameLower(), "finished", this.data.Finished)
//	//	logger.Debug("update quest.finished")
//	//}
//	//if _,ok := this.dirtyMap["Quests"]; ok {
//	//	db.GetPlayerDb().SaveComponentField(this.GetPlayerId(), this.GetNameLower(), "quests", this.data.Quests)
//	//	logger.Debug("update quest.quests")
//	//}
//	//this.dirtyMap = make(map[string]struct{})
//	//return nil,true
//	mapData := make(map[string]interface{})
//	mapData["finished"] = this.data.Finished
//	mapData["quests"] = this.data.Quests
//	return mapData,internal.Plain
//}
//
//func (this *Quest) Load(data interface{}, fromCache bool) error {
//	switch t := data.(type) {
//	case *pb.Quest:
//		// 加载明文数据
//		this.data = t
//		this.checkData()
//		logger.Debug("%v", this.data)
//	case []byte:
//		// 反序列化
//		err := internal.LoadWithProto(data, this.data)
//		this.checkData()
//		logger.Debug("%v", this.data)
//		return err
//	}
//	return nil
//}

func (this *Quest) checkData() {
	if this.data.Quests == nil {
		this.data.Quests = make(map[int32]*pb.QuestData)
	}
}

// 事件接口
func (this *Quest) OnEvent(event interface{}) {
	switch event.(type) {
	case *internal.EventPlayerEntryGame:
		//// 测试代码
		//if len(this.data.Finished) == 0 {
		//	this.data.Finished = append(this.data.Finished,1)
		//	this.SetDirty("finished", this.data.Finished)
		//	logger.Debug("finished:%v", this.data.Finished)
		//}
		//questData := &pb.QuestData{CfgId: int32(rand.Intn(1000)), Progress: rand.Int31n(100)}
		//this.data.Quests[questData.CfgId] = questData
		//this.SetDirty("quests", this.data.Finished)
		//logger.Debug("add quest:%v", questData)
	}
}