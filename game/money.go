package game

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/logger"
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

func (this *Player) GetMoney() *Money {
	return this.GetComponentByName(ComponentNameMoney).(*Money)
}

func (this *Money) IncCoin(coin int32) {
	this.Data.Coin += coin
	this.SetDirty()
}

func (this *Money) IncDiamond(diamond int32) {
	this.Data.Diamond += diamond
	this.SetDirty()
}

// 请求加coin的消息回调
// 这种格式写的函数可以自动注册客户端消息回调
func (this *Money) OnCoinReq(req *pb.CoinReq) {
	logger.Debug("OnCoinReq:%v", req)
	this.IncCoin(req.GetAddCoin())
	this.GetPlayer().Send(&pb.CoinRes{
		TotalCoin: this.Data.GetCoin(),
	})
}
