package loginserver

import (
	"log/slog"
	"math/rand"
	"context"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
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
		slog.Debug("loginRes", "accountName", loginRes.AccountName, "accountId", account.GetId(), "gameServer", loginRes.GameServer, "error", errorCode)
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
		if account.Id == 0 {
			errorCode = pb.ErrorCode_ErrorCode_NotReg
			return
			// 密码安全说明:客户端在传输前已完成加密(如RSA+AES混合加密),传到服务器的并非明文
			// 服务器存储和比较的都是加密后的值,因此这里用 != 直接比较是安全的
		} else if req.GetPassword() != account.GetPassword() {
			errorCode = pb.ErrorCode_ErrorCode_PasswordError
			return
		}
	}
	// 检查服务器维护状态:维护中仅白名单账号可进入
	if cache.IsMaintenanceMode() && !cache.IsWhitelistedAccount(account.GetId()) {
		errorCode = pb.ErrorCode_ErrorCode_Maintenance
		return
	}
	// 检查账号是否被封禁
	if banRecord := db.GetBanRecord(db.BanTargetTypeAccount, account.GetId()); banRecord != nil {
		loginRes.BanReason = banRecord.GetReason()
		if banRecord.Duration == 0 {
			loginRes.BanDeadline = 0 // 永久封禁
		} else {
			loginRes.BanDeadline = banRecord.BanTime + banRecord.Duration
		}
		errorCode = pb.ErrorCode_ErrorCode_Banned
		return
	}
	loginRes.AccountId = account.Id
	loginRes.LoginSession = cache.NewLoginSession(account)
	if loginRes.LoginSession == "" {
		errorCode = pb.ErrorCode_ErrorCode_DbErr
		return
	}
	onlinePlayerId, gameServerId := cache.GetOnlineAccount(account.GetId())
	if onlinePlayerId > 0 {
		// 如果该账号还在游戏中,则需要先将其清理下线
		slog.Error("exist online account", "accountId", account.GetId(), "playerId", onlinePlayerId, "gameServerId", gameServerId)
		if gameServerId > 0 {
			if _loginServer.GetServerList().GetServerInfo(gameServerId) == nil {
				// 目标游戏服已宕机(不在服务器列表中),直接清理 Redis 缓存,防止玩家永久"卡号"
				// 条件释放:值匹配才删,防止清理瞬间该玩家恰好重新登录到其他服后误删新记录
				cache.RemoveOnlinePlayer(onlinePlayerId, gameServerId)
				cache.RemoveOnlineAccount(account.GetId(), onlinePlayerId, gameServerId)
				slog.Error("RemoveOnlinePlayer for crashed server", "accountId", account.GetId(), "playerId", onlinePlayerId, "gameServerId", gameServerId)
			} else {
				// 游戏服在线,异步通知踢人
				// 不能用同步 Rpc:此处运行在 gate 连接的收包协程中,Rpc 会阻塞整个收包协程,
				// 导致同一网关上所有其他玩家的消息投递被阻塞
				// 踢人完成后客户端通过 PlayerEntryGameReq 登录,若踢人尚未完成则 AddOnlineAccount 失败返回 TryLater,客户端重试即可
				cmd := network.GetCommandByProto(new(pb.KickPlayerReq))
				internal.GetServerList().Send(gameServerId, PacketCommand(cmd), &pb.KickPlayerReq{
					AccountId: account.GetId(),
					PlayerId:  onlinePlayerId,
				})
			}
		}
	}
	// 返回区服列表(读internal包的进程内存快照,与GameServer共用同一套区服数据)
	loginRes.Regions = internal.GetAllRegions()
	// 查询该账号在各区服的角色概要信息
	loginRes.RegionRoles = queryAccountRegionRoles(account.GetId())
	// 分配一个游戏服给客户端连接
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

// 查询账号在各区服的角色概要信息(两段式,全程无分片广播)
// 第一段:account_player映射表按_id前缀查该账号所有区服的(playerId,regionId)——直达;
// 第二段:player表按_id($in)批量查角色名/等级——每个_id各自路由到目标分片,仍然直达。
// (原实现对player表按AccountId查询,分片集群下会广播到所有分片)
//
// 顶层字段Name是建角时以map key写入,保留大写开头;
// 组件字段是proto明文存储(db:"plain"),无bson tag,由mongo driver转为全小写
// (组件名如BaseInfo保留原名,其内部proto字段如level为小写)
func queryAccountRegionRoles(accountId int64) []*pb.RegionRoleInfo {
	// 第一段:映射表查账号在各区服的角色
	accounts, err := db.FindAccountPlayersByAccount(accountId)
	if err != nil {
		slog.Error("queryAccountRegionRoles accountMap", "err", err)
		return nil
	}
	if len(accounts) == 0 {
		return nil
	}
	playerIds := make([]int64, 0, len(accounts))
	for _, acc := range accounts {
		playerIds = append(playerIds, acc.PlayerId)
	}
	// 第二段:player表按_id批量查角色名与等级
	// 只查询需要的字段: _id(玩家id), Name, BaseInfo.level
	playerCol := db.GetPlayerDb().(*gentity.MongoCollectionPlayer).GetCollection()
	projection := bson.D{
		{"_id", 1},
		{db.PlayerName, 1},
		// BaseInfo以db:"plain"存储,内部proto字段名为小写level
		// (字面量与game.ComponentNameBaseInfo对应,loginserver不依赖game包避免引入组件注册副作用)
		{"BaseInfo", bson.D{{"level", 1}}},
	}
	cursor, err := playerCol.Find(context.Background(),
		bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: playerIds}}}},
		options.Find().SetProjection(projection))
	if err != nil {
		slog.Error("queryAccountRegionRoles", "err", err)
		return nil
	}
	var results []bson.M
	if err = cursor.All(context.Background(), &results); err != nil {
		slog.Error("queryAccountRegionRoles cursor", "err", err)
		return nil
	}
	// playerId -> 角色名/等级
	type roleSummary struct {
		name  string
		level int32
	}
	summaryMap := make(map[int64]*roleSummary, len(results))
	for _, doc := range results {
		summary := &roleSummary{}
		if idVal, ok := doc["_id"]; ok {
			if v, ok := idVal.(int64); ok {
				summaryMap[v] = summary
			} else if v, ok := idVal.(int32); ok {
				summaryMap[int64(v)] = summary
			} else {
				continue
			}
		} else {
			continue
		}
		if nameVal, ok := doc[db.PlayerName]; ok {
			if v, ok := nameVal.(string); ok {
				summary.name = v
			}
		}
		if baseInfoVal, ok := doc["BaseInfo"]; ok {
			// mongo-driver解码到bson.M时,嵌套文档的实际类型是bson.D(数组形式)而非bson.M,
			// 仅断言bson.M会静默失败导致Level恒为0,两种类型都必须兼容
			switch baseInfo := baseInfoVal.(type) {
			case bson.M:
				if levelVal, ok := baseInfo["level"]; ok {
					summary.level = toInt32(levelVal)
				}
			case bson.D:
				for _, elem := range baseInfo {
					if elem.Key == "level" {
						summary.level = toInt32(elem.Value)
					}
				}
			}
		}
	}
	// 组装:regionId来自映射表,角色名/等级来自player表
	regionRoles := make([]*pb.RegionRoleInfo, 0, len(accounts))
	for _, acc := range accounts {
		info := &pb.RegionRoleInfo{
			PlayerId: acc.PlayerId,
			RegionId: acc.RegionId,
		}
		if summary, ok := summaryMap[acc.PlayerId]; ok {
			info.PlayerName = summary.name
			info.Level = summary.level
		}
		regionRoles = append(regionRoles, info)
	}
	return regionRoles
}

// toInt32 安全地将 bson.M 中的值转为 int32
// MongoDB 驱动解码时数值可能是 int32/int64/float64,直接断言会 panic
func toInt32(v any) int32 {
	switch val := v.(type) {
	case int32:
		return val
	case int64:
		return int32(val)
	case float64:
		return int32(val)
	}
	return 0
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
		Id:  newAccountId,
		Name: req.GetAccountName(),
		// 存储的是客户端加密后的值,非明文密码(加密由客户端负责)
		Password: req.GetPassword(),
	}
	// _id=账号名:同名注册由MongoDB原子拒绝(E11000),分片集群下同样成立
	// (与单机环境依靠Name唯一索引的语义一致,但防线从应用层索引下沉到了主键路由)
	err, isDuplicateKey := _loginServer.GetAccountDb().InsertEntity(account.Name, AccountSaveData(account))
	if err != nil {
		account.Id = 0
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
	res.AccountId = account.Id
}

