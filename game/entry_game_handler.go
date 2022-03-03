package game

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/gameplayer"
	"github.com/fish-tennis/gserver/util"
)

// 玩家进游戏服
func onPlayerEntryGameReq(connection Connection, packet *ProtoPacket) {
	req := packet.Message().(*pb.PlayerEntryGameReq)
	if connection.GetTag() != nil {
		connection.Send(PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
			Result: "HasLogin",
		})
		return
	}
	// 验证LoginSession
	if !cache.VerifyLoginSession(req.GetAccountId(), req.GetLoginSession()) {
		connection.Send(PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
			Result: "SessionError",
		})
		return
	}
	playerData := &pb.PlayerData{}
	hasData,err := db.GetPlayerDb().FindPlayerByAccountId(req.GetAccountId(), req.GetRegionId(), playerData)
	if err != nil {
		connection.Send(PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
			Result: "DbError",
		})
		logger.Error(err.Error())
		return
	}
	var entryPlayer *gameplayer.Player
	isReconnect := false
	if !hasData {
		connection.Send(PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
			Result: "NoPlayer",
			AccountId: req.GetAccountId(),
			RegionId: req.GetRegionId(),
		})
		return
	} else {
		// 检查该账号是否已经有对应的在线玩家
		entryPlayer = _gameServer.GetPlayer(playerData.Id)
		if entryPlayer != nil {
			// 重连
			isReconnect = true
			logger.Debug("reconnect %v %v", entryPlayer.GetId(), entryPlayer.GetName())
		} else {
			entryPlayer = gameplayer.CreatePlayerFromData(playerData)
		}
	}
	if !isReconnect {
		// 分布式游戏服必须保证一个账号同时只在一个游戏服上登录,防止写数据覆盖
		// 通过redis做缓存来实现账号的"独占性"
		if !cache.AddOnlineAccount(entryPlayer.GetAccountId(), entryPlayer.GetId(), _gameServer.GetServerId()) {
			// 该账号已经在另一个游戏服上登录了
			_,gameServerId := cache.GetOnlinePlayer(entryPlayer.GetId())
			logger.Error("exist online account:%v playerId:%v gameServerId:%v",
				entryPlayer.GetAccountId(), entryPlayer.GetId(), gameServerId)
			if gameServerId > 0 {
				// 通知目标游戏服踢掉玩家
				_gameServer.SendToServer(gameServerId, PacketCommand(pb.CmdInner_Cmd_KickPlayer), &pb.KickPlayer{
					AccountId: entryPlayer.GetAccountId(),
					PlayerId:  entryPlayer.GetId(),
				})
			}
			// 通知客户端稍后重新登录
			connection.Send(PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
				Result: "TryLater",
			})
			return
		}
		// 加入在线玩家表
		_gameServer.AddPlayer(entryPlayer)
	}
	// 玩家和连接设置关联
	connection.SetTag(entryPlayer.GetId())
	entryPlayer.SetConnection(connection)
	logger.Debug("entry entryPlayer:%v %v", entryPlayer.GetId(), entryPlayer.GetName())
	entryPlayer.SendPlayerEntryGameRes(&pb.PlayerEntryGameRes{
		Result:    "ok",
		AccountId: entryPlayer.GetAccountId(),
		PlayerId:  entryPlayer.GetId(),
		RegionId:  entryPlayer.GetRegionId(),
	})
	// 分发事件
	entryPlayer.FireEvent(&internal.EventPlayerEntryGame{IsReconnect: isReconnect})
}

// 创建角色
func onCreatePlayerReq(connection Connection, packet *ProtoPacket) {
	req := packet.Message().(*pb.CreatePlayerReq)
	if connection.GetTag() != nil {
		connection.Send(PacketCommand(pb.CmdLogin_Cmd_CreatePlayerRes), &pb.CreatePlayerRes{
			Result: "HasLogin",
		})
		return
	}
	// 验证LoginSession
	if !cache.VerifyLoginSession(req.GetAccountId(), req.GetLoginSession()) {
		connection.Send(PacketCommand(pb.CmdLogin_Cmd_CreatePlayerRes), &pb.CreatePlayerRes{
			Result: "SessionError",
		})
		return
	}
	playerData := &pb.PlayerData{
		Id: util.GenUniqueId(),
		Name: req.Name,
		AccountId: req.AccountId,
		RegionId: req.RegionId,
		BaseInfo: &pb.BaseInfo{
			Gender: req.Gender,
			Level: 1,
			Exp: 0,
		},
	}
	err,isDuplicateKey := db.GetPlayerDb().InsertEntity(playerData.Id, playerData)
	if err != nil {
		result := "DbError"
		if isDuplicateKey {
			result = "DuplicateName"
		}
		connection.Send(PacketCommand(pb.CmdLogin_Cmd_CreatePlayerRes), &pb.CreatePlayerRes{
			Result: result,
		})
		logger.Error("CreatePlayer result:%v err:%v", result, err)
		return
	}
	logger.Debug("CreatePlayer:%v %v", playerData.Id, playerData.Name)
	connection.Send(PacketCommand(pb.CmdLogin_Cmd_CreatePlayerRes), &pb.CreatePlayerRes{
		Result: "ok",
		AccountId: req.AccountId,
		RegionId: req.RegionId,
		Name: req.Name,
	})
}