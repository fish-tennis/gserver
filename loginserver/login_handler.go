package loginserver

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
	"math/rand"
)

// 客户端账号登录
func onLoginReq(connection Connection, packet Packet) {
	logger.Debug("onLoginReq:%v", packet.Message())
	req := packet.Message().(*pb.LoginReq)
	var errorCode pb.ErrorCode
	loginRes := &pb.LoginRes{
		AccountName: req.GetAccountName(),
		//AccountId:    account.XId,
		//LoginSession: loginSession,
	}
	account := &pb.Account{}
	defer func() {
		network.SendPacketAdaptWithError(connection, packet, loginRes, int32(errorCode))
		logger.Debug("%v(%v) -> %v err:%v", loginRes.AccountName, account.GetXId(), loginRes.GameServer, errorCode)
	}()
	err := _loginServer.getAccountData(req.GetAccountName(), account)
	if err != nil {
		errorCode = pb.ErrorCode_ErrorCode_DbErr
		return
	} else {
		if account.XId == 0 {
			errorCode = pb.ErrorCode_ErrorCode_NotReg
			return
		} else if req.GetPassword() != account.GetPassword() {
			errorCode = pb.ErrorCode_ErrorCode_PasswordError
			return
		}
	}
	loginRes.AccountId = account.XId
	loginRes.LoginSession = cache.NewLoginSession(account)
	onlinePlayerId, gameServerId := cache.GetOnlineAccount(account.GetXId())
	if onlinePlayerId > 0 {
		// 如果该账号还在游戏中,则需要先将其清理下线
		logger.Error("exist online account:%v playerId:%v gameServerId:%v",
			account.GetXId(), onlinePlayerId, gameServerId)
		if gameServerId > 0 {
			//// 有可能那台游戏服宕机了,就直接清理缓存,防止"卡号"
			//if _loginServer.GetServerList().GetServerInfo(gameServerId) == nil {
			//	cache.RemoveOnlinePlayer(onlinePlayerId, gameServerId)
			//	cache.RemoveOnlineAccount(account.GetId())
			//	LogError("RemoveOnlinePlayer account:%v playerId:%v gameServerId:%v",
			//		account.GetId(), onlinePlayerId, gameServerId)
			//}
			cmd := network.GetCommandByProto(new(pb.KickPlayerReq))
			internal.GetServerList().Send(gameServerId, PacketCommand(cmd), &pb.KickPlayerReq{
				AccountId: account.GetXId(),
				PlayerId:  onlinePlayerId,
			})
		}
	}
	// 分配一个游戏服给客户端连接
	gameServerInfo := selectGameServer(account)
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
func onAccountReg(connection Connection, packet Packet) {
	logger.Debug("onAccountReg:%v", packet.Message())
	var errorCode pb.ErrorCode
	req := packet.Message().(*pb.AccountReg)
	res := &pb.AccountRes{
		AccountName: req.GetAccountName(),
	}
	defer func() {
		network.SendPacketAdaptWithError(connection, packet, res, int32(errorCode))
	}()
	result := ""
	newAccountIdValue, err := db.GetKvDb().Inc(db.AccountIdKeyName, int64(1), true)
	if err != nil {
		errorCode = pb.ErrorCode_ErrorCode_DbErr
		logger.Error("onAccountReg err:%v", err)
		return
	}
	newAccountId := newAccountIdValue.(int64)
	account := &pb.Account{
		XId:      newAccountId,
		Name:     req.GetAccountName(),
		Password: req.GetPassword(),
	}
	accountMapData := map[string]any{
		db.UniqueIdName: account.XId, // mongodb _id特殊处理
		"Name":          account.Name,
		"Password":      account.Password,
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
		logger.Error("onAccountReg account:%v result:%v err:%v", account.Name, result, err.Error())
		return
	}
	res.AccountId = account.XId
}
