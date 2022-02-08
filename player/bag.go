package player

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"google.golang.org/protobuf/proto"
	"math"
)

// 一个简单的背包模块
type Bag struct {
	DataComponent
	data *pb.Bag
}

func NewBag(player *Player, bytes []byte) *Bag {
	component := &Bag{
		DataComponent: DataComponent{
			BaseComponent: BaseComponent{
				Player: player,
				Name: "Bag",
			},
		},
	}
	if len(bytes) == 0 {
		data := &pb.Bag{}
		component.data = data
	} else {
		component.Deserialize(bytes)
	}
	component.checkData()
	logger.Debug("%v itemsCount:%v uniqueItemsCount:%v", component.GetPlayerId(), len(component.data.CountItems), len(component.data.UniqueItems))
	return component
}

// 需要保存的数据
func (this *Bag) Serialize(forCache bool) interface{} {
	// 演示proto序列化后存储到数据库
	// 优点:占用空间少,读取数据快,游戏模块大多采用这种方式
	// 缺点:数据库语言无法直接操作字段
	data,err := proto.Marshal(this.data)
	if err != nil {
		logger.Error("%v", err)
		return nil
	}
	return data
}

func (this *Bag) Deserialize(bytes []byte) error {
	data := &pb.Bag{}
	err := proto.Unmarshal(bytes, data)
	if err != nil {
		logger.Error("%v", err)
		return err
	}
	this.data = data
	this.checkData()
	return nil
}

// 事件接口
func (this *Bag) OnEvent(event interface{}) {
	switch event.(type) {
	case *internal.EventPlayerEntryGame:
		// 测试代码
		if len(this.data.CountItems) == 0 {
			this.AddItem(1,2)
		}
		if len(this.data.UniqueItems) == 0 {
			uniqueItem := &pb.UniqueItem{UniqueId: util.GenUniqueId(), ItemCfgId: 1001}
			this.AddUniqueItem(uniqueItem)
		}
	}
}

func (this *Bag) checkData() {
	if this.data.CountItems == nil {
		this.data.CountItems = make(map[int32]int32)
	}
	if this.data.UniqueItems == nil {
		this.data.UniqueItems = make(map[int64]*pb.UniqueItem)
	}
}

func (this *Bag) AddItem(itemCfgId,addCount int32) {
	if addCount <= 0 {
		return
	}
	curCount,ok := this.data.CountItems[itemCfgId]
	if ok {
		// 检查数值溢出
		if int64(curCount) + int64(addCount) > math.MaxInt32 {
			curCount = math.MaxInt32
		}
	} else {
		curCount = addCount
	}
	this.data.CountItems[itemCfgId] = curCount
	this.SetDirty()
}

func (this *Bag) AddUniqueItem(uniqueItem *pb.UniqueItem) {
	if _,ok := this.data.UniqueItems[uniqueItem.UniqueId]; !ok {
		this.data.UniqueItems[uniqueItem.UniqueId] = uniqueItem
		this.SetDirty()
	}
}