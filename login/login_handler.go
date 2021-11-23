package login

import (
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/pb"
)

// 客户端账号登录
func onLoginReq(connection gnet.Connection, packet *gnet.ProtoPacket) {
	gnet.LogDebug("onLoginReq:%v", packet.Message())
	req := packet.Message().(*pb.LoginReq)
	// 测试
	result := ""
	if req.GetAccountName() == "test" && req.GetPassword() == "test" {
		result = "ok"
	}
	connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_LoginRes), &pb.LoginRes{
		AccountName: req.GetAccountName(),
		Result: result,
	})
}