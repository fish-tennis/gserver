// Code generated by proto_code_gen. DO NOT EDIT
// https://github.com/fish-tennis/proto_code_gen
package gameplayer

import (
 "github.com/fish-tennis/gnet"
 "github.com/fish-tennis/gserver/pb"
)

// 查看公会列表返回结果
func (this *Player) SendGuildListRes(packet *pb.GuildListRes) bool {
   return this.Send(gnet.PacketCommand(pb.CmdGuild_Cmd_GuildListRes), packet)
}

// 创建公会请求返回结果
func (this *Player) SendGuildCreateRes(packet *pb.GuildCreateRes) bool {
   return this.Send(gnet.PacketCommand(pb.CmdGuild_Cmd_GuildCreateRes), packet)
}

// 加入公会请求返回结果
func (this *Player) SendGuildJoinRes(packet *pb.GuildJoinRes) bool {
   return this.Send(gnet.PacketCommand(pb.CmdGuild_Cmd_GuildJoinRes), packet)
}

// 同意加入公会返回结果
func (this *Player) SendGuildJoinAgreeRes(packet *pb.GuildJoinAgreeRes) bool {
   return this.Send(gnet.PacketCommand(pb.CmdGuild_Cmd_GuildJoinAgreeRes), packet)
}

// 查看公会数据返回结果
func (this *Player) SendGuildDataViewRes(packet *pb.GuildDataViewRes) bool {
   return this.Send(gnet.PacketCommand(pb.CmdGuild_Cmd_GuildDataViewRes), packet)
}

// 玩家登录游戏服回复
func (this *Player) SendPlayerEntryGameRes(packet *pb.PlayerEntryGameRes) bool {
   return this.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), packet)
}

// 请求加coin的返回结果
// @Player表示是服务器上的玩家对象发给客户端的消息,工具会生成相应的辅助代码
func (this *Player) SendCoinRes(packet *pb.CoinRes) bool {
   return this.Send(gnet.PacketCommand(pb.CmdMoney_Cmd_CoinRes), packet)
}

