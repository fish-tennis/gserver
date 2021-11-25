package login

import (
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/pb"
	"time"
)

// 客户端账号登录
func onLoginReq(connection gnet.Connection, packet *gnet.ProtoPacket) {
	gnet.LogDebug("onLoginReq:%v", packet.Message())
	req := packet.Message().(*pb.LoginReq)
	result := ""
	account := &pb.Account{}
	hasData,err := loginServer.GetAccountDb().FindString(req.GetAccountName(),account)
	if err != nil {
		result = err.Error()
	} else {
		if !hasData {
			result = "not reg"
		} else if req.GetPassword() == account.GetPassword() {
			result = "ok"
		} else {
			result = "password not correct"
		}
	}
	connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_LoginRes), &pb.LoginRes{
		AccountName: req.GetAccountName(),
		Result: result,
	})
}

// 注册账号
func onAccountReg(connection gnet.Connection, packet *gnet.ProtoPacket) {
	gnet.LogDebug("onAccountReg:%v", packet.Message())
	req := packet.Message().(*pb.AccountReg)
	result := ""
	account := &pb.Account{
		AccountId: time.Now().UnixNano(),
		AccountName: req.GetAccountName(),
		Password: req.GetPassword(),
	}
	err := loginServer.GetAccountDb().InsertInt64(account.GetAccountId(), account)
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
