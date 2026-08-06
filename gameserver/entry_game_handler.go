package gameserver

import (
	"log/slog"
	"time"

	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/game"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
)

// 玩家进游戏服的请求
// 在Connection的收包协程中调用
// 含DB查询(MongoDB)的进游逻辑投递到DB协程池异步执行,避免阻塞收包goroutine
func onPlayerEntryGameReq(connection Connection, packet Packet) {
	req := packet.Message().(*pb.PlayerEntryGameReq)
	accountId := req.GetAccountId()
	if !internal.SubmitDbTask(accountId, func() {
		processPlayerEntryGameReq(connection, packet, req)
	}) {
		// 协程池队列满,返回TryLater让客户端延迟重试
		network.SendPacketAdaptWithError(connection, packet, &pb.PlayerEntryGameRes{
			AccountId: req.AccountId,
			RegionId:  req.RegionId,
		}, int32(pb.ErrorCode_ErrorCode_TryLater))
		slog.Warn("DbWorkerPool full for onPlayerEntryGameReq", "accountId", accountId)
	}
}

// processPlayerEntryGameReq 进游请求的实际处理逻辑,在DB协程池中执行
func processPlayerEntryGameReq(connection Connection, packet Packet, req *pb.PlayerEntryGameReq) {
	res := &pb.PlayerEntryGameRes{
		AccountId: req.AccountId,
		RegionId:  req.RegionId,
	}
	var errorCode pb.ErrorCode
	var entryPlayer *game.Player
	// routedToRoutine 标记:内存命中的进游请求已投递到玩家协程处理,
	// 响应发送和进游逻辑都在协程内完成,defer不再重复处理
	routedToRoutine := false
	defer func() {
		if routedToRoutine {
			// 响应已在玩家协程的onEntryReconnect中发送,这里直接返回
			slog.Debug("onPlayerEntryGameReq routed to routine", "accountId", req.GetAccountId())
			return
		}
		network.SendPacketAdaptWithError(connection, packet, res, int32(errorCode))
		if errorCode == 0 && entryPlayer != nil {
			// 转到玩家协程中去处理
			cmd := network.GetCommandByProto(new(pb.PlayerEntryGameOk))
			entryPlayer.OnRecvPacket(NewProtoPacket(PacketCommand(cmd), &pb.PlayerEntryGameOk{
				IsReconnect: false,
			}))
		}
		slog.Debug("onPlayerEntryGameReq", "res", res, "error", errorCode)
	}()
	// DB协程池中执行,connection跨协程:先检查连接是否已断开
	// IsConnected是原子读,Close后返回false,避免后续操作已关闭的connection
	if !connection.IsConnected() {
		slog.Debug("processPlayerEntryGameReq connection closed", "accountId", req.GetAccountId())
		return
	}
	// HasLogin检查仅用于客户端直连模式(网关模式下connection是gate连接,tag不会是playerId)
	if !network.IsGatePacket(packet) && connection.GetTag() != nil {
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
		slog.Error("db error", "error", err)
		return
	}
	if playerId == 0 {
		errorCode = pb.ErrorCode_ErrorCode_NoPlayer
		return
	}
	// 检查该账号是否已经有对应的在线玩家
	entryPlayer = game.GetPlayer(playerId)
	if entryPlayer != nil {
		// 玩家已在内存中(保留期或在线),且请求已通过LoginSession验证,身份合法
		// 投递到玩家协程内绑定新连接、取消保留期、发送响应和进游同步
		// 响应在协程内发送,消除 GetPlayer 与 RemovePlayer 的 check-then-act 竞态:
		// 即使投递时玩家协程正在退出,TryPushMessage 返回 false,客户端不会误收成功响应
		if !entryPlayer.OnEntryReconnect(connection, network.IsGatePacket(packet), req, packet) {
			errorCode = pb.ErrorCode_ErrorCode_TryLater
			slog.Warn("player channel full for entry reconnect", "playerId", playerId)
			return
		}
		routedToRoutine = true
		slog.Debug("entry player reconnect", "playerId", entryPlayer.GetId())
		return
	}
	// 分布式游戏服必须保证一个账号同时只在一个游戏服上登录,防止写数据覆盖
	// 通过redis做缓存来实现账号的"独占性"
	if !cache.AddOnlineAccount(accountId, playerId, gentity.GetApplication().GetId()) {
		// 该账号已经在另一个游戏服上登录了
		_, gameServerId := cache.GetOnlinePlayer(playerId)
		slog.Error("exist online account", "accountId", accountId, "playerId", playerId, "gameServerId", gameServerId)
		if gameServerId > 0 {
			// 异步通知目标游戏服踢掉玩家
			// 不能用同步 Rpc:此处运行在 gate 连接的收包协程中,Rpc 会阻塞整个收包协程,
			// 导致同一网关上所有其他玩家的消息投递被阻塞
			// 客户端收到 TryLater 后会延迟重试,届时目标服踢人已完成,AddOnlineAccount 即可成功
			cmd := network.GetCommandByProto(new(pb.KickPlayerReq))
			sendOk := internal.GetServerList().Send(gameServerId, PacketCommand(cmd), &pb.KickPlayerReq{
				AccountId: accountId,
				PlayerId:  playerId,
			})
			if !sendOk {
				slog.Error("kick send failed", "accountId", accountId, "playerId", playerId, "gameServerId", gameServerId)
			}
		} else {
			// onlineaccount 存在但 onlineplayer 无有效 gameServerId
			// 不直接 RemoveOnlineAccount:无法区分是"残留数据"还是"另一个GS正在进游的中间态"
			// 若为后者,直接删除会破坏账号独占性(SetNX),导致同账号在两个GS同时在线,数据覆盖
			// 残留数据会由以下兜底机制清理:
			//   1. LoginServer:登录时检测目标服宕机则清理(LS.onLoginReq)
			//   2. onKickPlayer:收到踢人但player不在内存时清理残留(GS.onKickPlayer)
			//   3. repairCache:GameServer启动时修复宕机残留(RS.ResetOnlinePlayer)
			// 客户端收到 TryLater 重试即可,不会永久卡号
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
		slog.Error("db error", "error", err)
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
	if !entryPlayer.RunRoutine() {
		// 清理连接 tag,防止残留 playerId 导致后续消息路由异常
		if !network.IsGatePacket(packet) {
			connection.SetTag(nil)
		}
		game.GetPlayerMgr().RemovePlayer(entryPlayer)
		cache.RemoveOnlineAccount(accountId)
		errorCode = pb.ErrorCode_ErrorCode_TryLater
		slog.Error("RunRoutine failed", "playerId", entryPlayer.GetId())
		return
	}
	slog.Debug("entry player", "playerId", entryPlayer.GetId(), "name", entryPlayer.GetName())
	res.PlayerId = entryPlayer.GetId()
	res.PlayerName = entryPlayer.GetName()
	// 下发当前游戏服id,客户端保存后用于后续重连请求
	res.GameServerId = int32(gentity.GetApplication().GetId())
	//res.GuildData = entryPlayer.GetGuild().GetGuildData()
}

// 玩家重连游戏服的请求
// 在Connection的收包协程中调用
func onPlayerReconnectGameReq(connection Connection, packet Packet) {
	req := packet.Message().(*pb.PlayerReconnectGameReq)
	player := game.GetPlayer(req.PlayerId)
	if player == nil {
		// 玩家不在线(可能保留期已过或从未登录)
		res := &pb.PlayerReconnectGameRes{
			AccountId: req.AccountId,
			PlayerId:  req.PlayerId,
		}
		network.SendPacketAdaptWithError(connection, packet, res, int32(pb.ErrorCode_ErrorCode_ReconnectNeedRelogin))
		slog.Debug("onPlayerReconnectGameReq player nil", "playerId", req.PlayerId)
		return
	}
	// 投递到玩家协程执行,重连的校验、绑定连接、响应发送都在玩家协程内串行处理
	if !player.OnReconnect(connection, network.IsGatePacket(packet), req.GetReconnectSession(), packet) {
		network.SendPacketAdaptWithError(connection, packet, &pb.PlayerReconnectGameRes{
			AccountId: req.AccountId,
			PlayerId:  req.PlayerId,
		}, int32(pb.ErrorCode_ErrorCode_TryLater))
		slog.Warn("player channel full for reconnect", "playerId", req.PlayerId)
	}
}

// 创建角色
// 含DB操作(MongoDB KV自增ID + Insert),投递到DB协程池异步执行,避免阻塞收包goroutine
func onCreatePlayerReq(connection Connection, packet Packet) {
	req := packet.Message().(*pb.CreatePlayerReq)
	accountId := req.GetAccountId()
	if !internal.SubmitDbTask(accountId, func() {
		processCreatePlayerReq(connection, packet, req)
	}) {
		// 协程池队列满,返回TryLater让客户端延迟重试
		network.SendPacketAdaptWithError(connection, packet, &pb.CreatePlayerRes{
			AccountId: req.AccountId,
			Name:      req.Name,
			RegionId:  req.RegionId,
		}, int32(pb.ErrorCode_ErrorCode_TryLater))
		slog.Warn("DbWorkerPool full for onCreatePlayerReq", "accountId", accountId)
	}
}

// processCreatePlayerReq 创角请求的实际处理逻辑,在DB协程池中执行
func processCreatePlayerReq(connection Connection, packet Packet, req *pb.CreatePlayerReq) {
	slog.Debug("onCreatePlayerReq", "message", req)
	res := &pb.CreatePlayerRes{
		AccountId: req.AccountId,
		Name:      req.Name,
		RegionId:  req.RegionId,
	}
	var errorCode pb.ErrorCode
	defer func() {
		network.SendPacketAdaptWithError(connection, packet, res, int32(errorCode))
		slog.Debug("onCreatePlayerReq", "res", res, "error", errorCode)
	}()
	// DB协程池中执行,connection跨协程:先检查连接是否已断开
	if !connection.IsConnected() {
		slog.Debug("processCreatePlayerReq connection closed", "accountId", req.GetAccountId())
		return
	}
	// HasLogin检查仅用于客户端直连模式(网关模式下connection是gate连接,tag不会是playerId)
	if !network.IsGatePacket(packet) && connection.GetTag() != nil {
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
		slog.Error("onCreatePlayerReq error", "error", err)
		return
	}
	var newPlayerId int64
	switch idVal := newPlayerIdValue.(type) {
	case int64:
		newPlayerId = idVal
	case int32:
		newPlayerId = int64(idVal)
	case float64:
		newPlayerId = int64(idVal)
	default:
		errorCode = pb.ErrorCode_ErrorCode_DbErr
		slog.Error("onCreatePlayerReq invalid playerId type", "type", newPlayerIdValue, "val", newPlayerIdValue)
		return
	}
	playerData := &pb.PlayerData{
		XId:       newPlayerId,
		Name:      req.Name,
		AccountId: req.AccountId,
		RegionId:  req.RegionId,
		BaseInfo: &pb.BaseInfo{
			Gender: req.Gender,
			Level:  1,
			Exp:    0,
			// 记录角色创建时间(秒级时间戳),用于后续创角时长统计、老玩家回归等业务
			CreateTimestamp: time.Now().Unix(),
		},
	}
	newPlayer := game.CreatePlayerFromData(playerData)
	if newPlayer == nil {
		errorCode = pb.ErrorCode_ErrorCode_DbErr
		slog.Error("CreatePlayerFromDataErr", "accountId", req.AccountId, "playerData", playerData)
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
		slog.Error("CreatePlayer error", "errorCode", errorCode, "error", err, "playerData", playerData)
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

// gate特殊处理,暂时没有放在serverList里
func onGateHello(connection Connection, packet Packet) {
	hello := packet.Message().(*pb.ServerHello)
	slog.Info("onGateHello", "hello", hello, "connId", connection.GetConnectionId())
}
