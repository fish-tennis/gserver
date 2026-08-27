package loginserver

import (
	"log/slog"
	"math/rand"

	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
)

// 客户端账号登录
// 含DB查询(MongoDB账号查询 + Redis session),投递到DB协程池异步执行,避免阻塞收包goroutine
func onLoginReq(connection Connection, packet Packet) {
	req := packet.Message().(*pb.LoginReq)
	accountName := req.GetAccountName()
	if !internal.SubmitDbTaskByName(accountName, func() {
		processLoginReq(connection, packet, req)
	}) {
		// 协程池队列满,返回TryLater让客户端延迟重试
		network.SendPacketAdaptWithError(connection, packet, &pb.LoginRes{
			AccountName: accountName,
		}, int32(pb.ErrorCode_ErrorCode_TryLater))
		slog.Warn("DbWorkerPool full for onLoginReq", "accountName", accountName)
	}
}

// processLoginReq 登录请求的实际处理逻辑,在DB协程池中执行
func processLoginReq(connection Connection, packet Packet, req *pb.LoginReq) {
	slog.Debug("onLoginReq", "message", req)
	// 解析客户端真实IP:网关模式取请求体中由网关填充的ClientIp,直连模式从connection提取
	// 后续将用于玩家行为记录与分析,当前阶段先输出到日志
	clientIp := network.ResolveClientIp(connection, packet, req.GetClientIp())
	slog.Info("processLoginReq clientIp", "accountName", req.GetAccountName(), "clientIp", clientIp)
	var errorCode pb.ErrorCode
	loginRes := &pb.LoginRes{
		AccountName: req.GetAccountName(),
	}
	account := &pb.Account{}
	defer func() {
		network.SendPacketAdaptWithError(connection, packet, loginRes, int32(errorCode))
		slog.Debug("loginRes", "accountName", loginRes.AccountName, "accountId", account.GetXId(), "gameServer", loginRes.GameServer, "error", errorCode)
	}()
	// DB协程池中执行,connection跨协程:连接已断开则直接返回,避免无意义的DB查询
	if !connection.IsConnected() {
		slog.Debug("processLoginReq connection closed", "accountName", req.GetAccountName())
		return
	}
	err := _loginServer.getAccountData(req.GetAccountName(), account)
	if err != nil {
		errorCode = pb.ErrorCode_ErrorCode_DbErr
		return
	} else {
		if account.XId == 0 {
			errorCode = pb.ErrorCode_ErrorCode_NotReg
			return
			// 密码安全说明:客户端在传输前已完成加密(如RSA+AES混合加密),传到服务器的并非明文
			// 服务器存储和比较的都是加密后的值,因此这里用 != 直接比较是安全的
		} else if req.GetPassword() != account.GetPassword() {
			errorCode = pb.ErrorCode_ErrorCode_PasswordError
			return
		}
	}
	loginRes.AccountId = account.XId
	loginRes.LoginSession = cache.NewLoginSession(account)
	if loginRes.LoginSession == "" {
		errorCode = pb.ErrorCode_ErrorCode_DbErr
		return
	}
	onlinePlayerId, gameServerId := cache.GetOnlineAccount(account.GetXId())
	if onlinePlayerId > 0 {
		// 如果该账号还在游戏中,则需要先将其清理下线
		slog.Error("exist online account", "accountId", account.GetXId(), "playerId", onlinePlayerId, "gameServerId", gameServerId)
		if gameServerId > 0 {
			if _loginServer.GetServerList().GetServerInfo(gameServerId) == nil {
				// 目标游戏服已宕机(不在服务器列表中),直接清理 Redis 缓存,防止玩家永久"卡号"
				// 条件释放:值匹配才删,防止清理瞬间该玩家恰好重新登录到其他服后误删新记录
				cache.RemoveOnlinePlayer(onlinePlayerId, gameServerId)
				cache.RemoveOnlineAccount(account.GetXId(), onlinePlayerId, gameServerId)
				slog.Error("RemoveOnlinePlayer for crashed server", "accountId", account.GetXId(), "playerId", onlinePlayerId, "gameServerId", gameServerId)
			} else {
				// 游戏服在线,异步通知踢人
				// 不能用同步 Rpc:此处运行在 gate 连接的收包协程中,Rpc 会阻塞整个收包协程,
				// 导致同一网关上所有其他玩家的消息投递被阻塞
				// 踢人完成后客户端通过 PlayerEntryGameReq 登录,若踢人尚未完成则 AddOnlineAccount 失败返回 TryLater,客户端重试即可
				cmd := network.GetCommandByProto(new(pb.KickPlayerReq))
				internal.GetServerList().Send(gameServerId, PacketCommand(cmd), &pb.KickPlayerReq{
					AccountId: account.GetXId(),
					PlayerId:  onlinePlayerId,
				})
			}
		}
	}
	// 没有在线记录或目标服不可达,随机分配一个游戏服
	gameServerInfo := selectGameServer(account)
	if gameServerInfo == nil {
		errorCode = pb.ErrorCode_ErrorCode_TryLater
		return
	}
	loginRes.GameServer = &pb.GameServerInfo{
		ServerId:         gameServerInfo.GetServerId(),
		ClientListenAddr: gameServerInfo.GetClientListenAddr(),
	}
}

// 选择一个游戏服给登录成功的客户端
// NOTE:可以在这里做游戏服的负载均衡
func selectGameServer(account *pb.Account) *pb.ServerInfo {
	gameServerInfos := _loginServer.GetServerList().GetServersByType(internal.ServerType_Game)
	if len(gameServerInfos) > 0 {
		// 作为演示,这里随机一个
		selectGameServerInfo := gameServerInfos[rand.Intn(len(gameServerInfos))]
		return selectGameServerInfo
	}
	return nil
}

// 注册账号
// 含DB操作(MongoDB KV自增ID + Insert),投递到DB协程池异步执行,避免阻塞收包goroutine
func onAccountReg(connection Connection, packet Packet) {
	req := packet.Message().(*pb.AccountReg)
	accountName := req.GetAccountName()
	if !internal.SubmitDbTaskByName(accountName, func() {
		processAccountReg(connection, packet, req)
	}) {
		// 协程池队列满,返回TryLater让客户端延迟重试
		network.SendPacketAdaptWithError(connection, packet, &pb.AccountRes{
			AccountName: accountName,
		}, int32(pb.ErrorCode_ErrorCode_TryLater))
		slog.Warn("DbWorkerPool full for onAccountReg", "accountName", accountName)
	}
}

// processAccountReg 注册账号的实际处理逻辑,在DB协程池中执行
func processAccountReg(connection Connection, packet Packet, req *pb.AccountReg) {
	slog.Debug("onAccountReg", "message", req)
	var errorCode pb.ErrorCode
	res := &pb.AccountRes{
		AccountName: req.GetAccountName(),
	}
	defer func() {
		network.SendPacketAdaptWithError(connection, packet, res, int32(errorCode))
	}()
	// DB协程池中执行,connection跨协程:连接已断开则直接返回,避免无意义的DB操作
	if !connection.IsConnected() {
		slog.Debug("processAccountReg connection closed", "accountName", req.GetAccountName())
		return
	}
	// "账号名+空密码"直接登录该账号(登录分支的密码比较 ""=="" 形同虚设)
	if req.GetPassword() == "" {
		errorCode = pb.ErrorCode_ErrorCode_PasswordError
		slog.Warn("processAccountReg empty password", "accountName", req.GetAccountName())
		return
	}
	result := ""
	newAccountIdValue, err := db.GetKvDb().Inc(db.AccountIdKeyName, int64(1), true)
	if err != nil {
		errorCode = pb.ErrorCode_ErrorCode_DbErr
		slog.Error("onAccountReg error", "error", err)
		return
	}
	var newAccountId int64
	switch idVal := newAccountIdValue.(type) {
	case int64:
		newAccountId = idVal
	case int32:
		newAccountId = int64(idVal)
	case float64:
		newAccountId = int64(idVal)
	default:
		errorCode = pb.ErrorCode_ErrorCode_DbErr
		slog.Error("onAccountReg invalid accountId type", "type", newAccountIdValue, "val", newAccountIdValue)
		return
	}
	account := &pb.Account{
		XId:  newAccountId,
		Name: req.GetAccountName(),
		// 存储的是客户端加密后的值,非明文密码(加密由客户端负责)
		Password: req.GetPassword(),
	}
	accountMapData := map[string]any{
		db.UniqueIdName: account.XId, // mongodb _id特殊处理
		"Name":          account.Name,
		"Password":      account.Password, // 客户端加密后的值,非明文
	}
	err, isDuplicateKey := _loginServer.GetAccountDb().InsertEntity(account.XId, accountMapData)
	if err != nil {
		account.XId = 0
		if isDuplicateKey {
			errorCode = pb.ErrorCode_ErrorCode_NameDuplicate
			result = "AccountNameDuplicate"
		} else {
			result = "DbError"
			errorCode = pb.ErrorCode_ErrorCode_DbErr
		}
		slog.Error("onAccountReg error", "account", account.Name, "result", result, "error", err.Error())
		return
	}
	res.AccountId = account.XId
}
