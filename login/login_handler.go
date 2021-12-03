package login

import (
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/pb"
	"math/rand"
	"time"
)

// 客户端账号登录
func onLoginReq(connection gnet.Connection, packet *gnet.ProtoPacket) {
	gnet.LogDebug("onLoginReq:%v", packet.Message())
	req := packet.Message().(*pb.LoginReq)
	result := ""
	loginSession := ""
	account := &pb.Account{}
	hasData,err := loginServer.GetAccountDb().FindAccount(req.GetAccountName(),account)
	if err != nil {
		result = err.Error()
	} else {
		if !hasData {
			result = "not reg"
		} else if req.GetPassword() == account.GetPassword() {
			result = "ok"
			loginSession = cache.NewLoginSession(account)
		} else {
			result = "password not correct"
		}
	}
	loginRes := &pb.LoginRes{
		Result: result,
		AccountName: req.GetAccountName(),
		AccountId: account.GetId(),
		LoginSession: loginSession,
	}
	if result == "ok" {
		onlinePlayerId := cache.GetOnlineAccount(account.GetId())
		if onlinePlayerId > 0 {
			// 如果该账号还在游戏中,则需要先将其清理下线
			gameServerId := cache.GetOnlinePlayerGameServerId(onlinePlayerId)
			LogError("exist online account:%v playerId:%v gameServerId:%v",
				account.GetId(), onlinePlayerId, gameServerId)
			if gameServerId > 0 {
				// 有可能那台游戏服宕机了,就直接清理缓存,防止"卡号"
				if loginServer.GetServerList().GetServerInfo(gameServerId) == nil {
					cache.RemoveOnlinePlayer(onlinePlayerId)
					cache.RemoveOnlineAccount(account.GetId())
					LogError("RemoveOnlinePlayer account:%v playerId:%v gameServerId:%v",
						account.GetId(), onlinePlayerId, gameServerId)
				}
				loginServer.SendToServer(gameServerId, Cmd(pb.CmdInner_Cmd_KickPlayer), &pb.KickPlayer{
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
		gnet.LogDebug("%v -> %v", account.Name, loginRes.GameServer)
	}
	connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_LoginRes), loginRes)
}

// 选择一个游戏服给登录成功的客户端
// NOTE:可以在这里做游戏服的负载均衡
func selectGameServer(account *pb.Account) *pb.ServerInfo {
	gameServerInfos := loginServer.GetServerList().GetServersByType("game")
	if len(gameServerInfos) > 0 {
		// 作为演示,这里随机一个
		selectGameServerInfo := gameServerInfos[rand.Intn(len(gameServerInfos))]
		return selectGameServerInfo
	}
	return nil
}

// 注册账号
func onAccountReg(connection gnet.Connection, packet *gnet.ProtoPacket) {
	gnet.LogDebug("onAccountReg:%v", packet.Message())
	req := packet.Message().(*pb.AccountReg)
	result := ""
	account := &pb.Account{
		Id: time.Now().UnixNano(),
		Name: req.GetAccountName(),
		Password: req.GetPassword(),
	}
	err := loginServer.GetAccountDb().InsertAccount(account)
	if err != nil {
		result = err.Error()
	} else {
		result = "ok"
	}
	connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_LoginRes), &pb.LoginRes{
		AccountName: req.GetAccountName(),
		Result: result,
	})
}
