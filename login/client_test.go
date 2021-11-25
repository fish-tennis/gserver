package login

import (
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"testing"
)

// 模拟一个客户端进行登录操作
func TestClientLogin(t *testing.T)  {
	connectionConfig := gnet.ConnectionConfig{
		SendPacketCacheCap: 100,
		SendBufferSize:     1024*10,
		RecvBufferSize:     1024*10,
		MaxPacketSize:      1024*10,
		RecvTimeout:        0,
		HeartBeatInterval:  5,
		WriteTimeout:       0,
	}
	netMgr := gnet.GetNetMgr()
	clientCodec := gnet.NewProtoCodec(nil)
	clientHandler := &testClientHandler{
		DefaultConnectionHandler: *gnet.NewDefaultConnectionHandler(clientCodec),
		exitNotify: make(chan struct{}, 1),
	}
	clientHandler.RegisterHeartBeat(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), func() proto.Message {
		return &pb.HeartBeatReq{}
	})
	clientHandler.Register(gnet.PacketCommand(pb.CmdLogin_Cmd_LoginRes), clientHandler.onLoginRes, func() proto.Message {
		return &pb.LoginRes{}
	})
	if netMgr.NewConnector("127.0.0.1:10002", connectionConfig, clientCodec, clientHandler) == nil {
		panic("connect error")
	}

	select {
	case <-clientHandler.exitNotify:
		gnet.LogDebug("testClient exit")
		break
	}
}

type testClientHandler struct {
	gnet.DefaultConnectionHandler
	exitNotify chan struct{}
}

func (this *testClientHandler) OnConnected(connection gnet.Connection, success bool) {
	if !success {
		this.exitNotify <- struct{}{}
		return
	}
	connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_LoginReq), &pb.LoginReq{
		AccountName: "test",
		Password: "test",
	})
}

func (this *testClientHandler) onLoginRes(connection gnet.Connection, packet *gnet.ProtoPacket) {
	gnet.LogDebug("onLoginRes:%v", packet.Message())
	res := packet.Message().(*pb.LoginRes)
	if res.GetResult() == "not reg" {
		connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_AccountReg), &pb.AccountReg{
			AccountName: "test",
			Password: "test",
		})
	} else {
		connection.Close()
		this.exitNotify <- struct{}{}
	}
}