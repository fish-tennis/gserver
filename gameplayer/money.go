package gameplayer

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

// 编译期检查是否实现了Saveable接口
// https://github.com/uber-go/guide/blob/master/style.md#verify-interface-compliance
var _ internal.Saveable = (*Money)(nil)

// 玩家的钱财组件
type Money struct {
	PlayerDataComponent
	// 该字段必须导出(首字母大写)
	// 使用struct tag来标记该字段需要存数据库,可以设置存储字段名(proto格式存mongo时,使用全小写格式)
	Data *pb.Money `db:"money"`
}

func NewMoney(player *Player) *Money {
	component := &Money{
		PlayerDataComponent: *NewPlayerDataComponent(player, "Money"),
		Data: &pb.Money{
			Coin:    0,
			Diamond: 0,
		},
	}
	return component
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
// 这种格式写的函数可以自动注册消息回调
func (this *Money) OnCoinReq(req *pb.CoinReq) {
	logger.Debug("OnCoinReq:%v", req)
	this.IncCoin(req.GetAddCoin())
	this.GetPlayer().SendCoinRes(&pb.CoinRes{
		TotalCoin: this.Data.GetCoin(),
	})
}

// 请求加coin的消息回调
// 这种格式写的函数可以被proto_code_gen工具自动注册消息回调
func OnCoinReq(this *Money, req *pb.CoinReq) {
	logger.Debug("OnCoinReq:%v", req)
	this.IncCoin(req.GetAddCoin())
	this.GetPlayer().SendCoinRes(&pb.CoinRes{
		TotalCoin: this.Data.GetCoin(),
	})
}
