package login

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"math/rand"
)

// 客户端账号登录
func onLoginReq(connection Connection, packet *ProtoPacket) {
	logger.Debug("onLoginReq:%v", packet.Message())
	req := packet.Message().(*pb.LoginReq)
	result := ""
	loginSession := ""
	account := &pb.Account{}
	hasData,err := _loginServer.GetAccountDb().FindAccount(req.GetAccountName(),account)
	if err != nil {
		result = err.Error()
	} else {
		if !hasData {
			result = "NotReg"
		} else if req.GetPassword() == account.GetPassword() {
			result = "ok"
			loginSession = cache.NewLoginSession(account)
		} else {
			result = "PasswordError"
		}
	}
	loginRes := &pb.LoginRes{
		Result: result,
		AccountName: req.GetAccountName(),
		AccountId: account.GetId(),
		LoginSession: loginSession,
	}
	if result == "ok" {
		onlinePlayerId,gameServerId := cache.GetOnlineAccount(account.GetId())
		if onlinePlayerId > 0 {
			// 如果该账号还在游戏中,则需要先将其清理下线
			logger.Error("exist online account:%v playerId:%v gameServerId:%v",
				account.GetId(), onlinePlayerId, gameServerId)
			if gameServerId > 0 {
				//// 有可能那台游戏服宕机了,就直接清理缓存,防止"卡号"
				//if _loginServer.GetServerList().GetServerInfo(gameServerId) == nil {
				//	cache.RemoveOnlinePlayer(onlinePlayerId, gameServerId)
				//	cache.RemoveOnlineAccount(account.GetId())
				//	LogError("RemoveOnlinePlayer account:%v playerId:%v gameServerId:%v",
				//		account.GetId(), onlinePlayerId, gameServerId)
				//}
				_loginServer.SendToServer(gameServerId, PacketCommand(pb.CmdInner_Cmd_KickPlayer), &pb.KickPlayer{
					AccountId: account.GetId(),
					PlayerId: onlinePlayerId,
				})
			}
		}
		// 分配一个游戏服给客户端连接
		gameServerInfo := selectGameServer(account)
		loginRes.GameServer = &pb.GameServerInfo{
			ServerId: gameServerInfo.GetServerId(),
			ClientListenAddr: gameServerInfo.GetClientListenAddr(),
		}
		logger.Debug("%v(%v) -> %v", account.Name, account.Id, loginRes.GameServer)
	}
	connection.Send(PacketCommand(pb.CmdLogin_Cmd_LoginRes), loginRes)
}

// 选择一个游戏服给登录成功的客户端
// NOTE:可以在这里做游戏服的负载均衡
func selectGameServer(account *pb.Account) *pb.ServerInfo {
	gameServerInfos := _loginServer.GetServerList().GetServersByType("game")
	if len(gameServerInfos) > 0 {
		// 作为演示,这里随机一个
		selectGameServerInfo := gameServerInfos[rand.Intn(len(gameServerInfos))]
		return selectGameServerInfo
	}
	return nil
}

// 注册账号
func onAccountReg(connection Connection, packet *ProtoPacket) {
	logger.Debug("onAccountReg:%v", packet.Message())
	req := packet.Message().(*pb.AccountReg)
	result := "ok"
	account := &pb.Account{
		Id: util.GenUniqueId(),
		Name: req.GetAccountName(),
		Password: req.GetPassword(),
	}
	err,isDuplicateKey := _loginServer.GetAccountDb().InsertAccount(account)
	if err != nil {
		account.Id = 0
		if isDuplicateKey {
			result = "AccountNameDuplicate"
		} else {
			result = "DbError"
		}
		logger.Error("onAccountReg account:%v result:%v err:%v", account.Name, result, err.Error())
	}
	connection.Send(PacketCommand(pb.CmdLogin_Cmd_AccountRes), &pb.AccountRes{
		Result: result,
		AccountName: account.Name,
		AccountId: account.Id,
	})
}
