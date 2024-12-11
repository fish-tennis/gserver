package gameserver

import (
	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/game"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
)

// 玩家进游戏服的请求
// 在Connection的收包协程中调用
func onPlayerEntryGameReq(connection Connection, packet Packet) {
	req := packet.Message().(*pb.PlayerEntryGameReq)
	res := &pb.PlayerEntryGameRes{
		AccountId: req.AccountId,
		RegionId:  req.RegionId,
	}
	var errorCode pb.ErrorCode
	var entryPlayer *game.Player
	isReconnect := false
	defer func() {
		network.SendPacketAdaptWithError(connection, packet, res, int32(errorCode))
		if errorCode == 0 && entryPlayer != nil {
			// 转到玩家协程中去处理
			entryPlayer.OnRecvPacket(NewProtoPacket(PacketCommand(pb.CmdServer_Cmd_PlayerEntryGameOk), &pb.PlayerEntryGameOk{
				IsReconnect: isReconnect,
			}))
		}
		logger.Debug("onPlayerEntryGameReq:%v err:%v", res, errorCode)
	}()
	if connection.GetTag() != nil {
		errorCode = pb.ErrorCode_ErrorCode_HasLogin
		return
	}
	accountId := req.GetAccountId()
	// 验证LoginSession
	if !cache.VerifyLoginSession(accountId, req.GetLoginSession()) {
		errorCode = pb.ErrorCode_ErrorCode_SessionError
		return
	}
	playerId, err := db.GetPlayerDb().FindPlayerIdByAccountId(accountId, req.GetRegionId())
	//hasData,err := db.GetPlayerDb().FindPlayerByAccountId(req.GetAccountId(), req.GetRegionId(), playerData)
	if err != nil {
		errorCode = pb.ErrorCode_ErrorCode_DbErr
		logger.Error(err.Error())
		return
	}
	if playerId == 0 {
		errorCode = pb.ErrorCode_ErrorCode_NoPlayer
		return
	}
	// 检查该账号是否已经有对应的在线玩家
	entryPlayer = game.GetPlayer(playerId)
	if entryPlayer != nil {
		// 重连
		isReconnect = true
		entryPlayer.SetConnection(connection, network.IsGatePacket(packet))
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
				kickReply := new(pb.KickPlayerRes)
				rpcErr := internal.GetServerList().Rpc(gameServerId, NewProtoPacketEx(pb.CmdServer_Cmd_KickPlayerReq, &pb.KickPlayerReq{
					AccountId: accountId,
					PlayerId:  playerId,
				}), kickReply)
				if rpcErr != nil {
					slog.Error("kick rpcErr", "accountId", accountId, "playerId", playerId,
						"gameServerId", gameServerId, "rpcErr", rpcErr)
				}
			} else {
				// TODO: RemoveOnlineAccount?
			}
			// 通知客户端稍后重新登录
			errorCode = pb.ErrorCode_ErrorCode_TryLater
			return
		}
		playerData := &pb.PlayerData{}
		hasData, err := db.GetPlayerDb().FindEntityById(playerId, playerData)
		if err != nil {
			cache.RemoveOnlineAccount(accountId)
			errorCode = pb.ErrorCode_ErrorCode_DbErr
			logger.Error(err.Error())
			return
		}
		if !hasData {
			cache.RemoveOnlineAccount(accountId)
			errorCode = pb.ErrorCode_ErrorCode_NoPlayer
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
			errorCode = pb.ErrorCode_ErrorCode_NoPlayer
			return
		}
		// 加入在线玩家表
		game.GetPlayerMgr().AddPlayer(entryPlayer)
		entryPlayer.SetConnection(connection, network.IsGatePacket(packet))
		// 开启玩家独立线程
		entryPlayer.RunRoutine()
	}
	logger.Debug("entry entryPlayer:%v %v", entryPlayer.GetId(), entryPlayer.GetName())
	res.PlayerId = entryPlayer.GetId()
	res.PlayerName = entryPlayer.GetName()
	//res.GuildData = entryPlayer.GetGuild().GetGuildData()
}

// 创建角色
func onCreatePlayerReq(connection Connection, packet Packet) {
	logger.Debug("onCreatePlayerReq %v", packet.Message())
	req := packet.Message().(*pb.CreatePlayerReq)
	res := &pb.CreatePlayerRes{
		AccountId: req.AccountId,
		Name:      req.Name,
		RegionId:  req.RegionId,
	}
	var errorCode pb.ErrorCode
	defer func() {
		network.SendPacketAdaptWithError(connection, packet, res, int32(errorCode))
		logger.Debug("onCreatePlayerReq:%v err:%v", res, errorCode)
	}()
	if connection.GetTag() != nil {
		errorCode = pb.ErrorCode_ErrorCode_HasLogin
		return
	}
	// 验证LoginSession
	if !cache.VerifyLoginSession(req.GetAccountId(), req.GetLoginSession()) {
		errorCode = pb.ErrorCode_ErrorCode_SessionError
		return
	}
	newPlayerIdValue, err := db.GetKvDb().Inc(db.PlayerIdKeyName, int64(1), true)
	if err != nil {
		errorCode = pb.ErrorCode_ErrorCode_DbErr
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
		errorCode = pb.ErrorCode_ErrorCode_DbErr
		logger.Error("CreatePlayerFromDataErr")
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
		errorCode = pb.ErrorCode_ErrorCode_DbErr
		if isDuplicateKey {
			errorCode = pb.ErrorCode_ErrorCode_NameDuplicate
		}
		logger.Error("CreatePlayer errorCode:%v err:%v playerData:%v", errorCode, err, playerData)
		return
	}
}

// gate转发的客户端掉线消息
func onClientDisconnect(connection Connection, packet Packet) {
	if gatePacket, ok := packet.(*network.GatePacket); ok {
		playerId := gatePacket.PlayerId()
		player := game.GetPlayer(playerId)
		if player == nil {
			return
		}
		slog.Info("onClientDisconnect", "playerId", playerId, "connId", connection.GetConnectionId())
		player.OnDisconnect(connection)
	}
}
