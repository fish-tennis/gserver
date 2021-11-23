package login

import (
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/common"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"time"
)

// 登录服
type LoginServer struct {
	common.BaseServer
	config *LoginServerConfig
}

// 登录服配置
type LoginServerConfig struct {
	// 客户端监听地址
	clientListenAddr string
	// 客户端连接配置
	clientConnConfig gnet.ConnectionConfig
}

func (this *LoginServer) Init() bool {
	if !this.BaseServer.Init() {
		return false
	}
	this.readConfig()
	netMgr := gnet.GetNetMgr()
	clientCodec := gnet.NewProtoCodec(nil)
	clientHandler := gnet.NewDefaultConnectionHandler(clientCodec)
	this.registerClientPacket(clientHandler)
	if netMgr.NewListener(this.config.clientListenAddr, this.config.clientConnConfig, clientCodec, clientHandler, nil) == nil {
		panic("listen failed")
		return false
	}
	gnet.LogDebug("LoginServer.Init")
	return true
}

func (this *LoginServer) Run() {
	this.BaseServer.Run()
	gnet.LogDebug("LoginServer.Run")
	this.BaseServer.WaitExit()
}

func (this *LoginServer) readConfig() {
	this.config = &LoginServerConfig{
		clientListenAddr: "127.0.0.1:10002",
	}
}

// 注册客户端消息回调
func (this *LoginServer) registerClientPacket(clientHandler *gnet.DefaultConnectionHandler) {
	clientHandler.Register(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), onHeartBeatReq, func() proto.Message {return &pb.HeartBeatReq{}})
	clientHandler.Register(gnet.PacketCommand(pb.CmdLogin_Cmd_LoginReq), onLoginReq, func() proto.Message {return &pb.LoginReq{}})
}

// 客户端心跳回复
func onHeartBeatReq(connection gnet.Connection, packet *gnet.ProtoPacket) {
	req := packet.Message().(*pb.HeartBeatReq)
	connection.Send(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatRes), &pb.HeartBeatRes{
		RequestTimestamp: req.GetTimestamp(),
		ResponseTimestamp: uint64(time.Now().UnixNano()/int64(time.Microsecond)),
	})
}
