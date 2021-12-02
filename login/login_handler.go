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
