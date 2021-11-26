package game

import (
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
)

// 玩家的钱财组件
type Money struct {
	BaseComponent
	data *pb.Money
}

func NewMoney(player *Player, playerData *pb.PlayerData) *Money {
	var data *pb.Money
	if len(playerData.Money) == 0 {
		data = &pb.Money{
			Coin: 0,
			Diamond: 0,
		}
	} else {
		data = &pb.Money{}
		proto.Unmarshal(playerData.Money, data)
	}
	gnet.LogDebug("%v", data)
	return &Money{
		BaseComponent: BaseComponent{
			Player: player,
		},
		data: data,
	}
}

func (this *Money) GetId() int {
	return 2
}

func (this *Money) GetName() string {
	return "money"
}

// 需要保存的数据
func (this *Money) DbData() interface{} {
	data,err := proto.Marshal(this.data)
	if err != nil {
		gnet.LogError("%v", err)
		return nil
	}
	return data
}