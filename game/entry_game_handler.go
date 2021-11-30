package game

import (
	"fmt"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/pb"
	"time"
)

// 玩家进游戏服
func onPlayerEntryGameReq(connection gnet.Connection, packet *gnet.ProtoPacket) {
	req := packet.Message().(*pb.PlayerEntryGameReq)
	if connection.GetTag() != nil {
		connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
			Result: "has login",
		})
		return
	}
	// 验证LoginSession
	if !cache.VerifyLoginSession(req.GetAccountId(), req.GetLoginSession()) {
		connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
			Result: "session error",
		})
		return
	}
	playerData := &pb.PlayerData{}
	hasData,err := gameServer.GetPlayerDb().FindPlayerByAccountId(req.GetAccountId(), req.GetRegionId(), playerData)
	if err != nil {
		connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
			Result: err.Error(),
		})
		gnet.LogError(err.Error())
		return
	}
	var player *Player
	if !hasData {
		// 新建
		playerData.Id = time.Now().UnixNano()
		playerData.Name = fmt.Sprintf("player%v", playerData.Id) // test
		playerData.AccountId = req.GetAccountId()
		playerData.RegionId = req.GetRegionId()
		player = CreatePlayerFromData(playerData)
		err = gameServer.GetPlayerDb().InsertPlayer(player.GetId(), playerData)
		if err != nil {
			connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
				Result: err.Error(),
			})
			gnet.LogError("%v", err)
			return
		}
		err = player.Save()
		if err != nil {
			connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
				Result: err.Error(),
			})
			gnet.LogError("%v", err)
			return
		}
		gnet.LogDebug("new player:%v", playerData.Id)
	} else {
		// 检查该账号是否已经有对应的在线玩家
		existPlayer := gameServer.GetPlayer(playerData.GetId())
		if existPlayer != nil {
			connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
				Result: "exist player",
			})
			return
		}
		player = CreatePlayerFromData(playerData)
	}
	// 加入在线玩家表
	connection.SetTag(player.GetId())
	gameServer.AddPlayer(player)
	gnet.LogDebug("entry player:%v", player)
	connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
		Result: "ok",
		AccountId: player.GetAccountId(),
		PlayerId: player.GetId(),
		RegionId: player.GetRegionId(),
	})
	//// 模拟修改玩家数据
	//baseInfo := player.GetComponent(1).(*BaseInfo)
	//baseInfo.IncExp(1)
	//money := player.GetComponent(2).(*Money)
	//money.IncCoin(1)
	//// 下线保存
	//err = player.Save()
	//if err != nil {
	//	gnet.LogError("%v", err)
	//	return
	//}
}
