package game

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/gameplayer"
	"github.com/fish-tennis/gserver/gen"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
)

func tryStartPlayerRoutine(connection Connection, packet *ProtoPacket) {
	req := packet.Message().(*pb.PlayerEntryGameReq)
	if connection.GetTag() != nil {
		connection.Send(PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
			Error: "HasLogin",
		})
		return
	}
	accountId := req.GetAccountId()
	// 验证LoginSession
	if !cache.VerifyLoginSession(accountId, req.GetLoginSession()) {
		connection.Send(PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
			Error: "SessionError",
		})
		return
	}
	playerId, err := db.GetPlayerDb().FindPlayerIdByAccountId(accountId, req.GetRegionId())
	//hasData,err := db.GetPlayerDb().FindPlayerByAccountId(req.GetAccountId(), req.GetRegionId(), playerData)
	if err != nil {
		connection.Send(PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
			Error: "DbError",
		})
		logger.Error(err.Error())
		return
	}
	var entryPlayer *gameplayer.Player
	isReconnect := false
	if playerId == 0 {
		connection.Send(PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
			Error:     "NoPlayer",
			AccountId: req.GetAccountId(),
			RegionId:  req.GetRegionId(),
		})
		return
	}

	// 检查该账号是否已经有对应的在线玩家
	entryPlayer = gameplayer.GetPlayerMgr().GetPlayer(playerId)
	if entryPlayer != nil {
		// 重连
		isReconnect = true
		logger.Debug("reconnect %v %v", entryPlayer.GetId(), entryPlayer.GetName())
	}

	if !isReconnect {
		// 分布式游戏服必须保证一个账号同时只在一个游戏服上登录,防止写数据覆盖
		// 通过redis做缓存来实现账号的"独占性"
		if !cache.AddOnlineAccount(accountId, playerId, GetServer().GetServerId()) {
			// 该账号已经在另一个游戏服上登录了
			_, gameServerId := cache.GetOnlinePlayer(playerId)
			logger.Error("exist online account:%v playerId:%v gameServerId:%v",
				accountId, playerId, gameServerId)
			if gameServerId > 0 {
				// 通知目标游戏服踢掉玩家
				gen.SendKickPlayer(gameServerId, &pb.KickPlayer{
					AccountId: accountId,
					PlayerId:  playerId,
				})
			}
			// 通知客户端稍后重新登录
			connection.Send(PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
				Error: "TryLater",
			})
			return
		}
		playerData := &pb.PlayerData{}
		hasData, err := db.GetPlayerDb().FindPlayerByAccountId(req.GetAccountId(), req.GetRegionId(), playerData)
		if err != nil {
			cache.RemoveOnlineAccount(accountId)
			connection.Send(PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
				Error: "DbError",
			})
			logger.Error(err.Error())
			return
		}
		if !hasData {
			cache.RemoveOnlineAccount(accountId)
			connection.Send(PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
				Error:     "NoPlayer",
				AccountId: req.GetAccountId(),
				RegionId:  req.GetRegionId(),
			})
			return
		}
		entryPlayer = gameplayer.CreatePlayerFromData(playerData)
		// 加入在线玩家表
		gameplayer.GetPlayerMgr().AddPlayer(entryPlayer)
		// 开启玩家独立线程
		entryPlayer.RunProcessRoutine()
	}

	// 玩家和连接设置关联
	connection.SetTag(entryPlayer.GetId())
	entryPlayer.SetConnection(connection)
	entryPlayer.OnRecvPacket(packet)
}

// 玩家进游戏服
func onPlayerEntryGameReq(connection Connection, packet *ProtoPacket) {
	if connection.GetTag() == nil {
		return
	}
	playerId, ok := connection.GetTag().(int64)
	if !ok {
		return
	}
	player := gameplayer.GetPlayerMgr().GetPlayer(playerId)
	if player == nil {
		return
	}

	gen.SendPlayerEntryGameRes(player, &pb.PlayerEntryGameRes{
		AccountId: player.GetAccountId(),
		PlayerId:  player.GetId(),
		RegionId:  player.GetRegionId(),
		GuildData: player.GetGuild().GetGuildData(),
	})
	// 分发事件
	player.FireEvent(&EventPlayerEntryGame{})
}

// 创建角色
func onCreatePlayerReq(connection Connection, packet *ProtoPacket) {
	logger.Debug("onCreatePlayerReq %v", packet.Message())
	req := packet.Message().(*pb.CreatePlayerReq)
	if connection.GetTag() != nil {
		connection.Send(PacketCommand(pb.CmdLogin_Cmd_CreatePlayerRes), &pb.CreatePlayerRes{
			Error: "HasLogin",
		})
		return
	}
	// 验证LoginSession
	if !cache.VerifyLoginSession(req.GetAccountId(), req.GetLoginSession()) {
		connection.Send(PacketCommand(pb.CmdLogin_Cmd_CreatePlayerRes), &pb.CreatePlayerRes{
			Error: "SessionError",
		})
		return
	}
	playerData := &pb.PlayerData{
		Id:        util.GenUniqueId(),
		Name:      req.Name,
		AccountId: req.AccountId,
		RegionId:  req.RegionId,
		BaseInfo: &pb.BaseInfo{
			Gender: req.Gender,
			Level:  1,
			Exp:    0,
		},
	}
	newPlayer := gameplayer.CreatePlayerFromData(playerData)
	newPlayerSaveData := make(map[string]interface{})
	newPlayerSaveData["id"] = playerData.Id
	newPlayerSaveData["name"] = playerData.Name
	newPlayerSaveData["accountid"] = playerData.AccountId
	newPlayerSaveData["regionid"] = playerData.RegionId
	GetEntitySaveData(newPlayer, newPlayerSaveData)
	err, isDuplicateKey := db.GetPlayerDb().InsertEntity(playerData.Id, newPlayerSaveData)
	if err != nil {
		result := "DbError"
		if isDuplicateKey {
			result = "DuplicateName"
		}
		connection.Send(PacketCommand(pb.CmdLogin_Cmd_CreatePlayerRes), &pb.CreatePlayerRes{
			Error: result,
			Name:  playerData.Name,
		})
		logger.Error("CreatePlayer result:%v err:%v playerData:%v", result, err, playerData)
		return
	}
	logger.Debug("CreatePlayer:%v %v", playerData.Id, playerData.Name)
	connection.Send(PacketCommand(pb.CmdLogin_Cmd_CreatePlayerRes), &pb.CreatePlayerRes{
		AccountId: req.AccountId,
		RegionId:  req.RegionId,
		Name:      req.Name,
	})
}
