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
	Data *pb.Money `db:"baseinfo"`
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

func (this *Money) DbData() (dbData interface{}, protoMarshal bool) {
	// 演示proto序列化后存储到数据库
	// 优点:占用空间少,读取数据快,游戏模块大多采用这种方式
	// 缺点:数据库语言无法直接操作字段
	return this.Data, true
}

func (this *Money) CacheData() interface{} {
	return this.Data
}

// 事件接口
func (this *Money) OnEvent(event interface{}) {
	switch v := event.(type) {
	case *internal.EventPlayerEntryGame:
		this.OnPlayerEntryGame(v)
		//// 测试倒计时,玩家的钱币每秒+1
		//this.player.GetTimerEntries().After(time.Second, func() time.Duration {
		//	this.IncCoin(1)
		//	//logger.Debug("timer IncCoin")
		//	return time.Second
		//})
		//this.player.GetTimerEntries().After(time.Second, func() time.Duration {
		//	this.IncDiamond(1)
		//	//logger.Debug("timer IncDiamond once")
		//	return 0
		//})
	}
}

// 事件处理
func (this *Money) OnPlayerEntryGame(eventPlayerEntryGame *internal.EventPlayerEntryGame) {
	logger.Debug("OnEvent:%v", eventPlayerEntryGame)
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
