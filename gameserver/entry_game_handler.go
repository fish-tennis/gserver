package gameserver

import (
	"unicode/utf8"
	"log/slog"
	"time"
	"strings"

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
	// 解析客户端真实IP:网关模式取请求体中由网关填充的ClientIp,直连模式从connection提取
	// 后续将用于玩家行为记录与分析,当前阶段先输出到日志
	clientIp := network.ResolveClientIp(connection, packet, req.GetClientIp())
	slog.Info("processPlayerEntryGameReq clientIp", "accountId", req.GetAccountId(), "clientIp", clientIp)
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
	if ok, _ := cache.VerifyLoginSession(accountId, req.GetLoginSession()); !ok {
		errorCode = pb.ErrorCode_ErrorCode_SessionError
		return
	}
	// 查映射表获取该账号在此区服的角色id
	// NOTE:不用player表的FindPlayerIdByAccountId——player分片集群下按AccountId查询
	// 会广播所有分片;映射表按_id直达(设计说明见db/account_player.go)
	playerId, err := db.FindPlayerIdByAccount(accountId, req.GetRegionId())
	if err != nil {
		errorCode = pb.ErrorCode_ErrorCode_DbErr
		slog.Error("db error", "error", err)
		return
	}
	if playerId == 0 {
		errorCode = pb.ErrorCode_ErrorCode_NoPlayer
		return
	}
	// 检查服务器维护状态:维护中仅白名单账号可进入
	if cache.IsMaintenanceMode() && !cache.IsWhitelistedAccount(accountId) {
		errorCode = pb.ErrorCode_ErrorCode_Maintenance
		return
	}
	// 检查区服维护状态:OnlyWhiteList 状态下仅白名单账号可进入
	// 区服数据改从internal内存快照读取(区服唯一源已收敛为MongoDB,启动时预加载+通知刷新);
	// err != nil 表示区服不在快照中,跳过区服维护检查,保持原先查询Redis缓存返回nil时的语义
	if region, err := internal.GetRegion(req.GetRegionId()); err == nil {
		if region.Status == pb.RegionStatus_RegionStatus_OnlyWhiteList && !cache.IsWhitelistedAccount(accountId) {
			errorCode = pb.ErrorCode_ErrorCode_Maintenance
			return
		}
	}
	// 检查账号是否被封禁:登录后GM封禁账号,LoginSession仍在有效期内,
	// 不重新登录直接进游会绕过LoginServer的账号封禁检查,此处兜底拦截
	if banRecord := db.GetBanRecord(db.BanTargetTypeAccount, accountId); banRecord != nil {
		res.BanReason = banRecord.GetReason()
		if banRecord.Duration == 0 {
			res.BanDeadline = 0 // 永久封禁
		} else {
			res.BanDeadline = banRecord.BanTime + banRecord.Duration
		}
		errorCode = pb.ErrorCode_ErrorCode_Banned
		return
	}
	// 检查玩家是否被封禁
	if banRecord := db.GetBanRecord(db.BanTargetTypePlayer, playerId); banRecord != nil {
		res.BanReason = banRecord.GetReason()
		if banRecord.Duration == 0 {
			res.BanDeadline = 0 // 永久封禁
		} else {
			res.BanDeadline = banRecord.BanTime + banRecord.Duration
		}
		errorCode = pb.ErrorCode_ErrorCode_Banned
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
		cache.RemoveOnlineAccount(accountId, playerId, gentity.GetApplication().GetId())
		errorCode = pb.ErrorCode_ErrorCode_DbErr
		slog.Error("db error", "error", err)
		return
	}
	if !hasData {
		cache.RemoveOnlineAccount(accountId, playerId, gentity.GetApplication().GetId())
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
		cache.RemoveOnlineAccount(accountId, playerId, gentity.GetApplication().GetId())
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
		cache.RemoveOnlineAccount(accountId, playerId, gentity.GetApplication().GetId())
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
	// 解析客户端真实IP:网关模式取请求体中由网关填充的ClientIp,直连模式从connection提取
	// 后续将用于玩家行为记录与分析,当前阶段先输出到日志
	clientIp := network.ResolveClientIp(connection, packet, req.GetClientIp())
	slog.Info("onPlayerReconnectGameReq clientIp", "playerId", req.GetPlayerId(), "clientIp", clientIp)
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
	// 检查服务器维护状态:维护中仅白名单账号可重连
	// 重连和进游一样需要拦截维护,否则被封禁/维护中的玩家可通过保留期内重连绕过限制
	if cache.IsMaintenanceMode() && !cache.IsWhitelistedAccount(req.GetAccountId()) {
		network.SendPacketAdaptWithError(connection, packet, &pb.PlayerReconnectGameRes{
			AccountId: req.AccountId,
			PlayerId:  req.PlayerId,
		}, int32(pb.ErrorCode_ErrorCode_Maintenance))
		slog.Info("onPlayerReconnectGameReq maintenance blocked", "playerId", req.PlayerId, "accountId", req.AccountId)
		return
	}
	// 检查区服维护状态:与进游的检查口径一致,OnlyWhiteList状态下仅白名单账号可重连,
	// 否则区服维护期间玩家可通过保留期内重连绕过区服级限制
	if region, err := internal.GetRegion(player.GetRegionId()); err == nil {
		if region.Status == pb.RegionStatus_RegionStatus_OnlyWhiteList && !cache.IsWhitelistedAccount(req.GetAccountId()) {
			network.SendPacketAdaptWithError(connection, packet, &pb.PlayerReconnectGameRes{
				AccountId: req.AccountId,
				PlayerId:  req.PlayerId,
			}, int32(pb.ErrorCode_ErrorCode_Maintenance))
			slog.Info("onPlayerReconnectGameReq region maintenance blocked", "playerId", req.PlayerId, "regionId", player.GetRegionId())
			return
		}
	}
	// 检查账号是否被封禁:账号级封禁需即时生效(与进游的兜底检查口径一致)
	if banRecord := db.GetBanRecord(db.BanTargetTypeAccount, req.GetAccountId()); banRecord != nil {
		res := &pb.PlayerReconnectGameRes{
			AccountId: req.AccountId,
			PlayerId:  req.PlayerId,
			BanReason: banRecord.GetReason(),
		}
		if banRecord.Duration == 0 {
			res.BanDeadline = 0 // 永久封禁
		} else {
			res.BanDeadline = banRecord.BanTime + banRecord.Duration
		}
		network.SendPacketAdaptWithError(connection, packet, res, int32(pb.ErrorCode_ErrorCode_Banned))
		slog.Info("onPlayerReconnectGameReq account banned blocked", "accountId", req.AccountId, "playerId", req.PlayerId)
		return
	}
	// 检查玩家是否被封禁:重连时也需要拦截,防止被封禁玩家利用保留期内重连绕过封禁
	if banRecord := db.GetBanRecord(db.BanTargetTypePlayer, req.GetPlayerId()); banRecord != nil {
		res := &pb.PlayerReconnectGameRes{
			AccountId: req.AccountId,
			PlayerId:  req.PlayerId,
			BanReason: banRecord.GetReason(),
		}
		if banRecord.Duration == 0 {
			res.BanDeadline = 0 // 永久封禁
		} else {
			res.BanDeadline = banRecord.BanTime + banRecord.Duration
		}
		network.SendPacketAdaptWithError(connection, packet, res, int32(pb.ErrorCode_ErrorCode_Banned))
		slog.Info("onPlayerReconnectGameReq banned blocked", "playerId", req.PlayerId, "accountId", req.AccountId)
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
	// 解析客户端真实IP:网关模式取请求体中由网关填充的ClientIp,直连模式从connection提取
	// 后续将用于玩家行为记录与分析,当前阶段先输出到日志
	clientIp := network.ResolveClientIp(connection, packet, req.GetClientIp())
	slog.Info("processCreatePlayerReq clientIp", "accountId", req.GetAccountId(), "playerName", req.GetName(), "clientIp", clientIp)
	// 角色名统一去首尾空格后再校验与存储:存储去空后名称,
	// 避免首尾带空格的名称入库后与重名判断产生歧义("小明 "与"小明"视觉相同却被视为不同名称)
	trimmed := strings.TrimSpace(req.GetName())
	res := &pb.CreatePlayerRes{
		AccountId: req.AccountId,
		Name:      trimmed,
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
	// 验证LoginSession,并从session缓存中取得账号名(LoginServer生成session时一并存入,免去MongoDB账号表查询)
	sessionOk, _ := cache.VerifyLoginSession(req.GetAccountId(), req.GetLoginSession())
	if !sessionOk {
		errorCode = pb.ErrorCode_ErrorCode_SessionError
		return
	}
	// 检查服务器维护状态:维护中仅白名单账号可操作
	if cache.IsMaintenanceMode() && !cache.IsWhitelistedAccount(req.GetAccountId()) {
		errorCode = pb.ErrorCode_ErrorCode_Maintenance
		return
	}
	// 检查账号是否被封禁:创角时还没有角色,检查账号级封禁
	if banRecord := db.GetBanRecord(db.BanTargetTypeAccount, req.GetAccountId()); banRecord != nil {
		errorCode = pb.ErrorCode_ErrorCode_Banned
		return
	}
	// 检查区服状态:区服数据必须存在于internal内存快照中,否则视为非法区服
	// ClosedRegistration 状态下禁止新建角色;OnlyWhiteList(维护)状态下仅白名单可操作
	region, err := internal.GetRegion(req.GetRegionId())
	if err != nil {
		slog.Warn("processCreatePlayerReq region not found", "regionId", req.GetRegionId())
		errorCode = pb.ErrorCode_ErrorCode_RegionIdError
		return
	}
	if region.Status == pb.RegionStatus_RegionStatus_ClosedRegistration {
		errorCode = pb.ErrorCode_ErrorCode_RegionClosedRegistration
		return
	}
	if region.Status == pb.RegionStatus_RegionStatus_OnlyWhiteList && !cache.IsWhitelistedAccount(req.GetAccountId()) {
		errorCode = pb.ErrorCode_ErrorCode_Maintenance
		return
	}
	// 本地基础校验:去空后为空或rune字符数不在1~12范围则拒绝
	// 用rune计数而非字节len():角色名含中文等多字节字符时应按视觉字符数限制
	// (纯空格名去空后rune数为0,同样落入<1分支)
	if nameLen := utf8.RuneCountInString(trimmed); nameLen < 1 || nameLen > 12 {
		errorCode = pb.ErrorCode_ErrorCode_NameInvalid
		return
	}
	// 映射表预检查:该账号在此区服已有角色则直接拒绝
	// 查account_player映射表(按_id直达)而非player表(按AccountId查询,分片集群下广播),
	// 映射是持久事实,与建角防重的原子裁决(InsertAccountPlayerMap)数据源一致;
	// 此检查同时挡住"建角成功后客户端重复请求/重试"导致的第二角色
	if existPlayerId, findErr := db.FindPlayerIdByAccount(req.GetAccountId(), req.GetRegionId()); findErr != nil {
		errorCode = pb.ErrorCode_ErrorCode_DbErr
		slog.Error("FindPlayerIdByAccount error", "error", findErr, "accountId", req.GetAccountId(), "regionId", req.GetRegionId())
		return
	} else if existPlayerId != 0 {
		errorCode = pb.ErrorCode_ErrorCode_PlayerAlreadyExist
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
	// 原子写入账号区服映射:一个账号在同一个区服只能创建1个角色
	// 映射表_id("{accountId}_{regionId}")的唯一性由MongoDB全局原子保证(设计说明见db/account_player.go),
	// 并发建角(双端登录/请求重试/恶意刷)时,只有insert成功的请求能继续,其余在数据库层被拒绝;
	// _id冲突有且只有一种含义:该账号在该区服已有角色(映射是持久事实,无需再查player表裁决)
	if ok, err := db.InsertAccountPlayerMap(req.GetAccountId(), req.GetRegionId(), newPlayerId); err != nil {
		errorCode = pb.ErrorCode_ErrorCode_DbErr
		slog.Error("InsertAccountPlayerMap error", "error", err, "accountId", req.GetAccountId(), "regionId", req.GetRegionId())
		return
	} else if !ok {
		errorCode = pb.ErrorCode_ErrorCode_PlayerAlreadyExist
		return
	}
	playerData := &pb.PlayerData{
		XId:       newPlayerId,
		Name:      trimmed,
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
		// 内存实体组装失败:同样回滚删除映射,与下方InsertEntity失败的回滚语义一致,
		// 否则残留映射会让该账号在该区服永远报PlayerAlreadyExist,需人工修复
		if rollbackErr := db.DeleteAccountPlayerMap(req.GetAccountId(), req.GetRegionId()); rollbackErr != nil {
			slog.Error("DeleteAccountPlayerMap rollback error", "error", rollbackErr, "accountId", req.GetAccountId(), "regionId", req.GetRegionId(), "playerId", playerData.XId)
		}
		errorCode = pb.ErrorCode_ErrorCode_DbErr
		slog.Error("CreatePlayerFromDataErr", "accountId", req.AccountId, "playerData", playerData)
		return
	}
	// NOTE:有一种理论可能: InsertAccountPlayerMap后,还没来得及执行InsertEntity就服务器崩溃了,会导致该账户无法在该区服创建角色
	// 几率非常非常低,留由人工审核后处理(手动执行DeleteAccountPlayerMap)
	newPlayerSaveData := make(map[string]interface{})
	newPlayerSaveData[db.UniqueIdName] = playerData.XId
	newPlayerSaveData[db.PlayerName] = playerData.Name
	newPlayerSaveData[db.PlayerAccountId] = playerData.AccountId
	newPlayerSaveData[db.PlayerRegionId] = playerData.RegionId
	gentity.GetEntitySaveData(newPlayer, newPlayerSaveData)
	err, isDuplicateKey := db.GetPlayerDb().InsertEntity(playerData.XId, newPlayerSaveData)
	if err != nil {
		// 映射已写入但player文档写入失败,回滚删除映射,
		// 让该账号无需人工干预即可立即重试建角
		if rollbackErr := db.DeleteAccountPlayerMap(req.GetAccountId(), req.GetRegionId()); rollbackErr != nil {
			// 回滚失败仅告警:残留映射会让该账号建角报PlayerAlreadyExist,
			// 属可人工排查修复的少数场景(与旧TTL方案的自动清理相比少了自愈,换取无残留误判)
			slog.Error("DeleteAccountPlayerMap rollback error", "error", rollbackErr, "accountId", req.GetAccountId(), "regionId", req.GetRegionId(), "playerId", playerData.XId)
		}
		errorCode = pb.ErrorCode_ErrorCode_DbErr
		if isDuplicateKey {
			errorCode = pb.ErrorCode_ErrorCode_NameDuplicate
		}
		slog.Error("CreatePlayer error", "errorCode", errorCode, "error", err, "playerData", playerData)
		return
	}
	// 建角成功:映射永久保留,作为"按账号查角色"的持久事实与建角防重依据
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
