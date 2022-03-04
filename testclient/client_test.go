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

var (
	accountName = "test3"
	accountPwd = "test3"
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

	gnet.SetLogLevel(gnet.DebugLevel)
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
	clientHandler.Register(gnet.PacketCommand(pb.CmdLogin_Cmd_AccountRes), clientHandler.onAccountRes, func() proto.Message {
		return &pb.AccountRes{}
	})
	if netMgr.NewConnector(ctx,"127.0.0.1:10002", connectionConfig, clientCodec, clientHandler, nil) == nil {
		panic("connect error")
	}
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
		AccountName: accountName,
		Password: accountPwd,
	})
}

// 账号登录回调
func (this *testLoginHandler) onLoginRes(connection gnet.Connection, packet *gnet.ProtoPacket) {
	logger.Debug("onLoginRes:%v", packet.Message())
	res := packet.Message().(*pb.LoginRes)
	if res.GetResult() == "NotReg" {
		connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_AccountReg), &pb.AccountReg{
			AccountName: accountName,
			Password: accountPwd,
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

// 注册账号回调
func (this *testLoginHandler) onAccountRes(connection gnet.Connection, packet *gnet.ProtoPacket) {
	logger.Debug("onAccountRes:%v", packet.Message())
	res := packet.Message().(*pb.AccountRes)
	if res.Result == "ok" {
		connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_LoginReq), &pb.LoginReq{
			AccountName: accountName,
			Password: accountPwd,
		})
	}
}


// 游戏服连接
type testGameHandler struct {
	gnet.DefaultConnectionHandler
	ctx context.Context
	loginRes *pb.LoginRes
	entryGameRes *pb.PlayerEntryGameRes
}

func (this *testGameHandler) RegisterPacket() {
	this.RegisterHeartBeat(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), func() proto.Message {
		return &pb.HeartBeatReq{
			Timestamp: uint64(util.GetCurrentMS()),
		}
	})
	this.Register(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatRes), this.onHeartBeatRes, func() proto.Message {return new(pb.HeartBeatRes)})
	this.Register(gnet.PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameRes), this.onPlayerEntryGameRes, func() proto.Message {return new(pb.PlayerEntryGameRes)})
	this.Register(gnet.PacketCommand(pb.CmdLogin_Cmd_CreatePlayerRes), this.onCreatePlayerRes, func() proto.Message {return new(pb.CreatePlayerRes)})
	this.Register(gnet.PacketCommand(pb.CmdMoney_Cmd_CoinRes), this.onCoinRes, func() proto.Message {return new(pb.CoinRes)})
	this.Register(gnet.PacketCommand(pb.CmdGuild_Cmd_GuildCreateRes), this.onGuildCreateRes, func() proto.Message {return new(pb.GuildCreateRes)})
	this.Register(gnet.PacketCommand(pb.CmdGuild_Cmd_GuildListRes), this.onGuildListRes, func() proto.Message {return new(pb.GuildListRes)})
	this.Register(gnet.PacketCommand(pb.CmdGuild_Cmd_GuildJoinRes), this.onGuildJoinRes, func() proto.Message {return new(pb.GuildJoinRes)})
	this.Register(gnet.PacketCommand(pb.CmdGuild_Cmd_RequestGuildDataRes), this.onRequestGuildDataRes, func() proto.Message {return new(pb.RequestGuildDataRes)})
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
	//logger.Debug("onHeartBeatRes:%v", packet.Message())
}

// 登录游戏服回调
func (this *testGameHandler) onPlayerEntryGameRes(connection gnet.Connection, packet *gnet.ProtoPacket) {
	logger.Debug("onPlayerEntryGameRes:%v", packet.Message())
	res := packet.Message().(*pb.PlayerEntryGameRes)
	if res.GetResult() == "ok" {
		this.entryGameRes = res
		// 玩家登录游戏服成功,模拟一个交互消息
		connection.Send(gnet.PacketCommand(pb.CmdMoney_Cmd_CoinReq), &pb.CoinReq{
			AddCoin: 1,
		})
		connection.Send(gnet.PacketCommand(pb.CmdGuild_Cmd_GuildListReq), &pb.GuildListReq{
			PageIndex: 0,
		})
		if res.GuildData.GuildId > 0 {
			// 已有公会 获取公会数据
			connection.Send(gnet.PacketCommand(pb.CmdGuild_Cmd_RequestGuildDataReq), &pb.RequestGuildDataReq{
			})
		}
		return
	}
	// 还没角色,则创建新角色
	if res.GetResult() == "NoPlayer" {
		connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_CreatePlayerReq), &pb.CreatePlayerReq{
			AccountId: this.loginRes.GetAccountId(),
			LoginSession: this.loginRes.GetLoginSession(),
			RegionId: 1,
			Name: accountName,
			Gender: 1,
		})
		return
	}
	// 登录遇到问题,服务器提示客户端稍后重试
	if res.GetResult() == "TryLater" {
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

// 创建角色回调
func (this *testGameHandler) onCreatePlayerRes(connection gnet.Connection, packet *gnet.ProtoPacket) {
	logger.Debug("onCreatePlayerRes:%v", packet.Message())
	res := packet.Message().(*pb.CreatePlayerRes)
	if res.Result == "ok" {
		connection.Send(gnet.PacketCommand(pb.CmdLogin_Cmd_PlayerEntryGameReq), &pb.PlayerEntryGameReq{
			AccountId: this.loginRes.GetAccountId(),
			LoginSession: this.loginRes.GetLoginSession(),
			RegionId: 1,
		})
	}
}

// 逻辑消息回调
func (this *testGameHandler) onCoinRes(connection gnet.Connection, packet *gnet.ProtoPacket) {
	logger.Debug("onCoinRes:%v", packet.Message())
}

func (this *testGameHandler) onGuildCreateRes(connection gnet.Connection, packet *gnet.ProtoPacket) {
	logger.Debug("onGuildCreateRes:%v", packet.Message())
}

func (this *testGameHandler) onGuildListRes(connection gnet.Connection, packet *gnet.ProtoPacket) {
	logger.Debug("onGuildListRes:%v", packet.Message())
	res := packet.Message().(*pb.GuildListRes)
	if len(res.GuildInfos) > 0 {
		if this.entryGameRes.GuildData.GuildId == 0 {
			connection.Send(gnet.PacketCommand(pb.CmdGuild_Cmd_GuildJoinReq), &pb.GuildJoinReq{
				Id: res.GuildInfos[0].Id,
			})
			logger.Debug("GuildJoinReq:%v", res.GuildInfos[0].Id)
		}
	} else {
		// 没有公会 就创建一个
		connection.Send(gnet.PacketCommand(pb.CmdGuild_Cmd_GuildCreateReq), &pb.GuildCreateReq{
			Name: "test",
			Intro: "empty",
		})
	}
}

func (this *testGameHandler) onGuildJoinRes(connection gnet.Connection, packet *gnet.ProtoPacket) {
	logger.Debug("onGuildJoinRes:%v", packet.Message())
}

func (this *testGameHandler) onRequestGuildDataRes(connection gnet.Connection, packet *gnet.ProtoPacket) {
	logger.Debug("onRequestGuildDataRes:%v", packet.Message())
	res := packet.Message().(*pb.RequestGuildDataRes)
	for _,v := range res.GuildData.JoinRequests {
		// 同意入会请求
		connection.Send(gnet.PacketCommand(pb.CmdGuild_Cmd_GuildJoinAgreeReq), &pb.GuildJoinAgreeReq{
			JoinPlayerId: v.PlayerId,
			IsAgree: true,
		})
	}
}
