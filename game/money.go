package game

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/pb"
)

const (
	// 组件名
	ComponentNameMoney = "Money"
)

// 利用go的init进行组件的自动注册
func init() {
	_playerComponentRegister.Register(ComponentNameMoney, 0, func(player *Player, _ any) gentity.Component {
		return &Money{
			PlayerDataComponent: *NewPlayerDataComponent(player, ComponentNameMoney),
			Data: &pb.Money{
				Coin:    0,
				Diamond: 0,
			},
		}
	})
}

// 玩家的钱财组件
type Money struct {
	PlayerDataComponent
	// 该字段必须导出(首字母大写)
	// 使用struct tag来标记该字段需要存数据库,可以设置存储字段名
	Data *pb.Money `db:""`
}

func (p *Player) GetMoney() *Money {
	return p.GetComponentByName(ComponentNameMoney).(*Money)
}

func (m *Money) SyncDataToClient() {
	m.GetPlayer().Send(&pb.MoneySync{
		Data: m.Data,
	})
}

func (m *Money) IncCoin(coin int32) {
	m.Data.Coin += coin
	m.SetDirty()
}

func (m *Money) IncDiamond(diamond int32) {
	m.Data.Diamond += diamond
	m.SetDirty()
}
