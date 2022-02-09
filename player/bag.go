package player

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"math"
)

// 一个简单的背包模块
type Bag struct {
	DataComponent
	data *pb.Bag
}

var _ internal.Saveable = (*Bag)(nil)

func NewBag(player *Player) *Bag {
	component := &Bag{
		DataComponent: DataComponent{
			BaseComponent: BaseComponent{
				Player: player,
				Name: "Bag",
			},
		},
		data: &pb.Bag{},
	}
	component.checkData()
	return component
}

// 需要保存的数据
func (this *Bag) Save(forCache bool) (saveData interface{}, isPlain bool) {
	return this.data,false
}

func (this *Bag) Load(data interface{}) error {
	err := internal.LoadWithProto(data, this.data)
	logger.Debug("%v", this.data)
	return err
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
			uniqueItem := &pb.UniqueItem{UniqueId: util.GenUniqueId(), CfgId: 1001}
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