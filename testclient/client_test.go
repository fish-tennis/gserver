package testclient

import (
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"google.golang.org/protobuf/proto"
	"testing"
)

var (
	exitNotify chan struct{}
)

// 模拟一个客户端
func TestClient(t *testing.T)  {
	connectionConfig := gnet.ConnectionConfig{
		SendPacketCacheCap: 100,
		SendBufferSize:     1024*10,
		RecvBufferSize:     1024*10,
		MaxPacketSize:      1024*10,
		RecvTimeout:        0,
		HeartBeatInterval:  5,
		WriteTimeout:       0,
	}
	exitNotify = make(chan struct{}, 1)
	netMgr := gnet.GetNetMgr()
	clientCodec := gnet.NewProtoCodec(nil)
	clientHandler := &testLoginHandler{
		DefaultConnectionHandler: *gnet.NewDefaultConnectionHandler(clientCodec),
		exitNotify: exitNotify,
	}
	clientHandler.RegisterHeartBeat(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), func() proto.Message {
		return &pb.HeartBeatReq{
			Timestamp: uint64(util.GetCurrentMS()),
		}
	})
	clientHandler.Register(gnet.PacketCommand(pb.CmdLogin_Cmd_LoginRes), clientHandler.onLoginRes, func() proto.Message {
		return &pb.LoginRes{}
	})
	if netMgr.NewConnector("127.0.0.1:10002", connectionConfig, clientCodec, clientHandler) == nil {
		panic("connect error")
	}

	select {
	case <-exitNotify:
		gnet.LogDebug("testClient exit")
		break
	}
}

// 账号登录连接
type testLoginHandler struct {
	gnet.DefaultConnectionHandler
	exitNotify chan struct{}
}

func (this *testLoginHandler) Exit() {
	this.exitNotify <- struct{}{}
}

func (this *testLoginHandler) OnConnected(connection gnet.Connection, success bool) {
	if !success {
		this.Exit()
		return
	}
	connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_LoginReq), &pb.LoginReq{
		AccountName: "test",
		Password: "test",
	})
}

func (this *testLoginHandler) onLoginRes(connection gnet.Connection, packet *gnet.ProtoPacket) {
	gnet.LogDebug("onLoginRes:%v", packet.Message())
	res := packet.Message().(*pb.LoginRes)
	if res.GetResult() == "not reg" {
		connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_AccountReg), &pb.AccountReg{
			AccountName: "test",
			Password: "test",
		})
	} else if res.GetResult() == "ok" {
		// 账号登录成功后,登录游戏服
		connection.Close()
		connectionConfig := gnet.ConnectionConfig{
			SendPacketCacheCap: 100,
			SendBufferSize:     1024*10,
			RecvBufferSize:     1024*10,
			MaxPacketSize:      1024*10,
			RecvTimeout:        0,
			HeartBeatInterval:  5,
			WriteTimeout:       0,
		}
		clientCodec := gnet.NewProtoCodec(nil)
		clientHandler := &testGameHandler{
			DefaultConnectionHandler: *gnet.NewDefaultConnectionHandler(clientCodec),
			loginRes: res,
			exitNotify: exitNotify,
		}
		clientHandler.RegisterPacket()
		if gnet.GetNetMgr().NewConnector(res.GetGameServer().GetClientListenAddr(), connectionConfig, clientCodec, clientHandler) == nil {
			panic("connect error")
		}
	} else {
		connection.Close()
		this.Exit()
	}
}

// 游戏服连接
type testGameHandler struct {
	gnet.DefaultConnectionHandler
	loginRes *pb.LoginRes
	exitNotify chan struct{}
}

func (this *testGameHandler) RegisterPacket() {
	this.RegisterHeartBeat(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), func() proto.Message {
		return &pb.HeartBeatReq{
			Timestamp: uint64(util.GetCurrentMS()),
		}
	})
	this.Register(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatRes), this.onHeartBeatRes, func() proto.Message {return &pb.HeartBeatRes{}})
	this.Register(gnet.PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), this.onPlayerEntryGameRes, func() proto.Message {return &pb.PlayerEntryGameRes{}})
}

func (this *testGameHandler) Exit() {
	this.exitNotify <- struct{}{}
}

func (this *testGameHandler) OnConnected(connection gnet.Connection, success bool) {
	if !success {
		this.Exit()
		return
	}
	connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameReq), &pb.PlayerEntryGameReq{
		AccountId: this.loginRes.GetAccountId(),
		LoginSession: this.loginRes.GetLoginSession(),
		RegionId: 1,
	})
}

func (this *testGameHandler) onHeartBeatRes(connection gnet.Connection, packet *gnet.ProtoPacket) {
	//gnet.LogDebug("onHeartBeatRes:%v", packet.Message())
}

func (this *testGameHandler) onPlayerEntryGameRes(connection gnet.Connection, packet *gnet.ProtoPacket) {
	gnet.LogDebug("onPlayerEntryGameRes:%v", packet.Message())
	//res := packet.Message().(*pb.PlayerEntryGameRes)
	//this.Exit()
	connection.Send(gnet.PacketCommand(pb.CmdMoney_Cmd_CoinReq), &pb.CoinReq{
		AddCoin: 1,
	})
}