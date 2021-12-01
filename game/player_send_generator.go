package game

import "github.com/fish-tennis/gserver/pb"

// 玩家发送不同的消息的封装,隐藏消息号,这里只是演示怎么样隐藏消息号
// 实际的应用中,只要proto文件按照统一的规范来写,这部分代码很容易通过工具来自动生成

func (this *Player) SendPlayerEntryGameRes(packet *pb.PlayerEntryGameRes) bool {
	return this.Send(Cmd(pb.CmdLogin_Cmd_PlayerEntryGameRes), packet)
}

func (this *Player) SendCoinRes(packet *pb.CoinRes) bool {
	return this.Send(Cmd(pb.CmdMoney_Cmd_CoinRes), packet)
}
