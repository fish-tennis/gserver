package game

import (
	"fmt"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"time"
)

// 玩家进游戏服
func onPlayerEntryGameReq(connection gnet.Connection, packet *ProtoPacket) {
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
		logger.Error(err.Error())
		return
	}
	var player *Player
	isReconnect := false
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
			logger.Error("%v", err)
			return
		}
		err = player.Save()
		if err != nil {
			connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
				Result: err.Error(),
			})
			logger.Error("%v", err)
			return
		}
		logger.Debug("new player:%v", playerData.Id)
	} else {
		// 检查该账号是否已经有对应的在线玩家
		player = gameServer.GetPlayer(playerData.GetId())
		if player != nil {
			// 重连
			isReconnect = true
			logger.Debug("reconnect %v", player.GetId())
			//connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
			//	Result: "exist player",
			//})
			//return
		} else {
			player = CreatePlayerFromData(playerData)
		}
	}
	if !isReconnect {
		// 分布式游戏服必须保证一个账号同时只在一个游戏服上登录,防止写数据覆盖
		// 通过redis做缓存来实现账号的"独占性"
		if !cache.AddOnlineAccount(player.GetAccountId(), player.GetId(), gameServer.GetServerId()) {
			// 该账号已经在另一个游戏服上登录了
			_,gameServerId := cache.GetOnlinePlayer(player.GetId())
			logger.Error("exist online account:%v playerId:%v gameServerId:%v",
				player.GetAccountId(), player.GetId(), gameServerId)
			if gameServerId > 0 {
				// 通知目标游戏服踢掉玩家
				gameServer.SendToServer(gameServerId, Cmd(pb.CmdInner_Cmd_KickPlayer), &pb.KickPlayer{
					AccountId: player.GetAccountId(),
					PlayerId: player.GetId(),
				})
			}
			// 通知客户端稍后重新登录
			connection.Send(Cmd(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
				Result: "try later",
			})
			return
		}
		// 加入在线玩家表
		gameServer.AddPlayer(player)
	}
	// 玩家和连接设置关联
	connection.SetTag(player.GetId())
	player.SetConnection(connection)
	logger.Debug("entry player:%v", player)
	player.SendPlayerEntryGameRes(&pb.PlayerEntryGameRes{
		Result: "ok",
		AccountId: player.GetAccountId(),
		PlayerId: player.GetId(),
		RegionId: player.GetRegionId(),
	})
	// 分发事件
	player.FireEvent(&EventPlayerEntryGame{isReconnect: isReconnect})
}
