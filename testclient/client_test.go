package testclient

import (
	"context"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
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
	ctx,cancel := context.WithCancel(context.Background())
	defer cancel()

	netMgr := gnet.GetNetMgr()
	clientCodec := gnet.NewProtoCodec(nil)
	clientHandler := &testLoginHandler{
		DefaultConnectionHandler: *gnet.NewDefaultConnectionHandler(clientCodec),
		ctx: ctx,
	}
	clientHandler.RegisterHeartBeat(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), func() proto.Message {
		return &pb.HeartBeatReq{
			Timestamp: uint64(util.GetCurrentMS()),
		}
	})
	clientHandler.Register(gnet.PacketCommand(pb.CmdLogin_Cmd_LoginRes), clientHandler.onLoginRes, func() proto.Message {
		return &pb.LoginRes{}
	})
	if netMgr.NewConnector(ctx,"127.0.0.1:10002", connectionConfig, clientCodec, clientHandler, nil) == nil {
		panic("connect error")
	}

	//// 监听系统的kill信号
	//signalKillNotify := make(chan os.Signal, 1)
	//signal.Notify(signalKillNotify, os.Interrupt, os.Kill, syscall.SIGTERM)
	////// windows系统上,加一个控制台输入,以方便调试
	////if runtime.GOOS == "windows" {
	////	go func() {
	////		consoleReader := bufio.NewReader(os.Stdin)
	////		for {
	////			lineBytes, _, _ := consoleReader.ReadLine()
	////			line := strings.ToLower(string(lineBytes))
	////			//gnet.LogDebug("line:%v", line)
	////			if line == "close" || line == "exit" {
	////				gnet.LogDebug("kill by console input")
	////				// 在windows系统模拟一个kill信号,以方便测试服务器退出流程
	////				signalKillNotify <- os.Kill
	////			}
	////		}
	////	}()
	////}
	//// 阻塞等待系统关闭信号
	//gnet.LogDebug("wait for kill signal")
	//select {
	//case <-signalKillNotify:
	//	gnet.LogDebug("signalKillNotify, cancel ctx")
	//	// 通知所有协程关闭,所有监听<-ctx.Done()的地方会收到通知
	//	cancel()
	//	break
	//}
	netMgr.Shutdown(true)
}

// 账号登录连接
type testLoginHandler struct {
	gnet.DefaultConnectionHandler
	ctx context.Context
}

func (this *testLoginHandler) OnConnected(connection gnet.Connection, success bool) {
	if !success {
		return
	}
	connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_LoginReq), &pb.LoginReq{
		AccountName: "test",
		Password: "test",
	})
}

func (this *testLoginHandler) onLoginRes(connection gnet.Connection, packet *gnet.ProtoPacket) {
	logger.Debug("onLoginRes:%v", packet.Message())
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
			ctx: this.ctx,
		}
		clientHandler.RegisterPacket()
		if gnet.GetNetMgr().NewConnector(this.ctx, res.GetGameServer().GetClientListenAddr(), connectionConfig, clientCodec, clientHandler, nil) == nil {
			panic("connect error")
		}
	} else {
		connection.Close()
	}
}

// 游戏服连接
type testGameHandler struct {
	gnet.DefaultConnectionHandler
	loginRes *pb.LoginRes
	ctx context.Context
}

func (this *testGameHandler) RegisterPacket() {
	this.RegisterHeartBeat(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), func() proto.Message {
		return &pb.HeartBeatReq{
			Timestamp: uint64(util.GetCurrentMS()),
		}
	})
	this.Register(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatRes), this.onHeartBeatRes, func() proto.Message {return new(pb.HeartBeatRes)})
	this.Register(gnet.PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), this.onPlayerEntryGameRes, func() proto.Message {return new(pb.PlayerEntryGameRes)})
	this.Register(gnet.PacketCommand(pb.CmdMoney_Cmd_CoinRes), this.onCoinRes, func() proto.Message {return new(pb.CoinRes)})
}

func (this *testGameHandler) OnConnected(connection gnet.Connection, success bool) {
	if !success {
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
	logger.Debug("onPlayerEntryGameRes:%v", packet.Message())
	res := packet.Message().(*pb.PlayerEntryGameRes)
	if res.GetResult() == "ok" {
		// 玩家登录游戏服成功,模拟一个交互消息
		connection.Send(gnet.PacketCommand(pb.CmdMoney_Cmd_CoinReq), &pb.CoinReq{
			AddCoin: 1,
		})
		return
	}
	// 登录遇到问题,服务器提示客户端稍后重试
	if res.GetResult() == "try later" {
		// 延迟重试
		time.AfterFunc(time.Second, func() {
			connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameReq), &pb.PlayerEntryGameReq{
				AccountId: this.loginRes.GetAccountId(),
				LoginSession: this.loginRes.GetLoginSession(),
				RegionId: 1,
			})
		})
	}
}

func (this *testGameHandler) onCoinRes(connection gnet.Connection, packet *gnet.ProtoPacket) {
	logger.Debug("onCoinRes:%v", packet.Message())
}