package game

import (
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/gen"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

// 玩家的钱财组件
type Money struct {
	PlayerDataComponent
	// 该字段必须导出(首字母大写)
	// 使用struct tag来标记该字段需要存数据库,可以设置存储字段名
	Data *pb.Money `db:"Money"`
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
// 这种格式写的函数可以自动注册客户端消息回调
func (this *Money) OnCoinReq(_ gnet.PacketCommand, req *pb.CoinReq) {
	logger.Debug("OnCoinReq:%v", req)
	this.IncCoin(req.GetAddCoin())
	gen.SendCoinRes(this.GetPlayer(), &pb.CoinRes{
		TotalCoin: this.Data.GetCoin(),
	})
}

// 请求加coin的消息回调
// 这种格式写的函数可以被proto_code_gen工具自动注册消息回调
func OnCoinReq(this *Money, req *pb.CoinReq) {
	logger.Debug("OnCoinReq:%v", req)
	this.IncCoin(req.GetAddCoin())
	gen.SendCoinRes(this.GetPlayer(), &pb.CoinRes{
		TotalCoin: this.Data.GetCoin(),
	})
}
