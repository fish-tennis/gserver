package gameserver

import (
	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/game"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
)

// 玩家进游戏服的请求
// 在Connection的收包协程中调用
func onPlayerEntryGameReq(connection Connection, packet Packet) {
	req := packet.Message().(*pb.PlayerEntryGameReq)
	if connection.GetTag() != nil {
		internal.SendPacketAdapt(connection, packet, PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
			Error: "HasLogin",
		})
		return
	}
	accountId := req.GetAccountId()
	// 验证LoginSession
	if !cache.VerifyLoginSession(accountId, req.GetLoginSession()) {
		internal.SendPacketAdapt(connection, packet, PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
			Error: "SessionError",
		})
		return
	}
	playerId, err := db.GetPlayerDb().FindPlayerIdByAccountId(accountId, req.GetRegionId())
	//hasData,err := db.GetPlayerDb().FindPlayerByAccountId(req.GetAccountId(), req.GetRegionId(), playerData)
	if err != nil {
		internal.SendPacketAdapt(connection, packet, PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
			Error: "DbError",
		})
		logger.Error(err.Error())
		return
	}
	var entryPlayer *game.Player
	isReconnect := false
	if playerId == 0 {
		internal.SendPacketAdapt(connection, packet, PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
			Error:     "NoPlayer",
			AccountId: req.GetAccountId(),
			RegionId:  req.GetRegionId(),
		})
		return
	}
	// 检查该账号是否已经有对应的在线玩家
	entryPlayer = game.GetPlayer(playerId)
	if entryPlayer != nil {
		// 重连
		isReconnect = true
		entryPlayer.SetConnection(connection, internal.IsGatePacket(packet))
		logger.Debug("reconnect %v %v", entryPlayer.GetId(), entryPlayer.GetName())
	}
	if !isReconnect {
		// 分布式游戏服必须保证一个账号同时只在一个游戏服上登录,防止写数据覆盖
		// 通过redis做缓存来实现账号的"独占性"
		if !cache.AddOnlineAccount(accountId, playerId, gentity.GetApplication().GetId()) {
			// 该账号已经在另一个游戏服上登录了
			_, gameServerId := cache.GetOnlinePlayer(playerId)
			logger.Error("exist online account:%v playerId:%v gameServerId:%v",
				accountId, playerId, gameServerId)
			if gameServerId > 0 {
				// 通知目标游戏服踢掉玩家
				kickReply := new(pb.KickPlayer)
				rpcErr := internal.GetServerList().Rpc(gameServerId, NewProtoPacketEx(pb.CmdInner_Cmd_KickPlayer, &pb.KickPlayer{
					AccountId: accountId,
					PlayerId:  playerId,
				}), kickReply)
				if rpcErr != nil {
					slog.Error("kick rpcErr", "accountId", accountId, "playerId", playerId,
						"gameServerId", gameServerId, "rpcErr", rpcErr)
				}
			}
			// 通知客户端稍后重新登录
			internal.SendPacketAdapt(connection, packet, PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
				Error: "TryLater",
			})
			return
		}
		playerData := &pb.PlayerData{}
		hasData, err := db.GetPlayerDb().FindEntityById(playerId, playerData)
		if err != nil {
			cache.RemoveOnlineAccount(accountId)
			internal.SendPacketAdapt(connection, packet, PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
				Error: "DbError",
			})
			logger.Error(err.Error())
			return
		}
		if !hasData {
			cache.RemoveOnlineAccount(accountId)
			internal.SendPacketAdapt(connection, packet, PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
				Error:     "NoPlayer",
				AccountId: req.GetAccountId(),
				RegionId:  req.GetRegionId(),
			})
			return
		}
		// Q:_id为什么不会赋值?
		// A:因为protobuf自动生成的struct tag,无法适配mongodb的_id字段
		// 解决方案: 使用工具生成自定义的struct tag,如github.com/favadi/protoc-go-inject-tag
		// 如果能生成下面这种struct tag,就可以直接把mongodb的_id的值赋值到playerData.XId了
		// XId int64 `protobuf:"varint,1,opt,name=_id,json=Id,proto3" json:"_id,omitempty" bson:"_id"`
		if playerData.XId == 0 {
			playerData.XId = playerId
		}
		entryPlayer = game.CreatePlayerFromData(playerData)
		if entryPlayer == nil {
			cache.RemoveOnlineAccount(accountId)
			internal.SendPacketAdapt(connection, packet, PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
				Error:     "DbError",
				AccountId: req.GetAccountId(),
				RegionId:  req.GetRegionId(),
			})
			return
		}
		// 加入在线玩家表
		game.GetPlayerMgr().AddPlayer(entryPlayer)
		entryPlayer.SetConnection(connection, internal.IsGatePacket(packet))
		// 开启玩家独立线程
		entryPlayer.RunRoutine()
	}
	logger.Debug("entry entryPlayer:%v %v", entryPlayer.GetId(), entryPlayer.GetName())
	internal.SendPacketAdapt(connection, packet, PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), &pb.PlayerEntryGameRes{
		AccountId:  entryPlayer.GetAccountId(),
		PlayerId:   entryPlayer.GetId(),
		RegionId:   entryPlayer.GetRegionId(),
		PlayerName: entryPlayer.GetName(),
		GuildData:  entryPlayer.GetGuild().GetGuildData(),
	})
	// 转到玩家协程中去处理
	entryPlayer.OnRecvPacket(NewProtoPacket(PacketCommand(pb.CmdBaseInfo_Cmd_PlayerEntryGameOk), &pb.PlayerEntryGameOk{
		IsReconnect: isReconnect,
	}))
}

// 创建角色
func onCreatePlayerReq(connection Connection, packet Packet) {
	logger.Debug("onCreatePlayerReq %v", packet.Message())
	req := packet.Message().(*pb.CreatePlayerReq)
	if connection.GetTag() != nil {
		internal.SendPacketAdapt(connection, packet, PacketCommand(pb.CmdLogin_Cmd_CreatePlayerRes), &pb.CreatePlayerRes{
			Error: "HasLogin",
		})
		return
	}
	// 验证LoginSession
	if !cache.VerifyLoginSession(req.GetAccountId(), req.GetLoginSession()) {
		internal.SendPacketAdapt(connection, packet, PacketCommand(pb.CmdLogin_Cmd_CreatePlayerRes), &pb.CreatePlayerRes{
			Error: "SessionError",
		})
		return
	}
	newPlayerIdValue, err := db.GetKvDb().Inc(db.PlayerIdKeyName, int64(1), true)
	if err != nil {
		internal.SendPacketAdapt(connection, packet, PacketCommand(pb.CmdLogin_Cmd_CreatePlayerRes), &pb.CreatePlayerRes{
			Error: "IdError",
		})
		logger.Error("onCreatePlayerReq err:%v", err)
		return
	}
	newPlayerId := newPlayerIdValue.(int64)
	playerData := &pb.PlayerData{
		XId:       newPlayerId,
		Name:      req.Name,
		AccountId: req.AccountId,
		RegionId:  req.RegionId,
		BaseInfo: &pb.BaseInfo{
			Gender: req.Gender,
			Level:  1,
			Exp:    0,
		},
	}
	newPlayer := game.CreatePlayerFromData(playerData)
	if newPlayer == nil {
		internal.SendPacketAdapt(connection, packet, PacketCommand(pb.CmdLogin_Cmd_CreatePlayerRes), &pb.CreatePlayerRes{
			Error: "DbError",
		})
		logger.Error("CreatePlayerFromData")
		return
	}
	newPlayerSaveData := make(map[string]interface{})
	newPlayerSaveData[db.UniqueIdName] = playerData.XId
	newPlayerSaveData[db.PlayerName] = playerData.Name
	newPlayerSaveData[db.PlayerAccountId] = playerData.AccountId
	newPlayerSaveData[db.PlayerRegionId] = playerData.RegionId
	gentity.GetEntitySaveData(newPlayer, newPlayerSaveData)
	err, isDuplicateKey := db.GetPlayerDb().InsertEntity(playerData.XId, newPlayerSaveData)
	if err != nil {
		result := "DbError"
		if isDuplicateKey {
			result = "DuplicateName"
		}
		internal.SendPacketAdapt(connection, packet, PacketCommand(pb.CmdLogin_Cmd_CreatePlayerRes), &pb.CreatePlayerRes{
			Error: result,
			Name:  playerData.Name,
		})
		logger.Error("CreatePlayer result:%v err:%v playerData:%v", result, err, playerData)
		return
	}
	logger.Debug("CreatePlayer:%v %v", playerData.XId, playerData.Name)
	internal.SendPacketAdapt(connection, packet, PacketCommand(pb.CmdLogin_Cmd_CreatePlayerRes), &pb.CreatePlayerRes{
		AccountId: req.AccountId,
		RegionId:  req.RegionId,
		Name:      req.Name,
	})
}

// gate转发的客户端掉线消息
func onClientDisconnect(connection Connection, packet Packet) {
	if gatePacket, ok := packet.(*internal.GatePacket); ok {
		playerId := gatePacket.PlayerId()
		player := game.GetPlayer(playerId)
		if player == nil {
			return
		}
		player.OnDisconnect(connection)
	}
}
