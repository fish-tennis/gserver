package game

import (
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
)

// 玩家的钱财组件
type Money struct {
	DataComponent
	data *pb.Money
}

func NewMoney(player *Player, bytes []byte) *Money {
	var data *pb.Money
	if len(bytes) == 0 {
		data = &pb.Money{
			Coin: 0,
			Diamond: 0,
		}
	} else {
		data = &pb.Money{}
		proto.Unmarshal(bytes, data)
	}
	gnet.LogDebug("%v", data)
	component := &Money{
		DataComponent: DataComponent{
			BaseComponent:BaseComponent{
				Player: player,
				id: 2,
				name: "money",
			},
		},
		data: data,
	}
	component.dataFun = component.DbData
	return component
}

// 需要保存的数据
func (this *Money) DbData() interface{} {
	// 演示proto序列化后存储到数据库
	// 优点:占用空间少,读取数据快,游戏模块大多采用这种方式
	// 缺点:数据库语言无法直接操作字段
	data,err := proto.Marshal(this.data)
	if err != nil {
		gnet.LogError("%v", err)
		return nil
	}
	return data
}