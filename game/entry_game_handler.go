package game

import (
	"fmt"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/pb"
	"time"
)

func onPlayerEntryGameReq(connection gnet.Connection, packet *gnet.ProtoPacket) {
	req := packet.Message().(*pb.PlayerEntryGameReq)
	// TODO:验证LoginSession
	// TODO:检查该账号是否已经有对应的在线玩家
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
		err = gameServer.GetPlayerDb().InsertPlayer(playerData.GetId(), player.playerData)
		if err != nil {
			connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
				Result: err.Error(),
			})
			gnet.LogError(err.Error())
		}
		gnet.LogDebug("new player:%v", playerData.Id)
	} else {
		player = CreatePlayerFromData(playerData)
	}
	// TODO:加入在线玩家表
	gnet.LogDebug("entry player:%v", player)
	connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
		Result: "ok",
		AccountId: player.GetAccountId(),
		PlayerId: player.GetId(),
		RegionId: player.GetRegionId(),
	})
}