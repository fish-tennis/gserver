package gate

import (
	"context"
	"encoding/json"
	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
	"math/rand"
	"os"
	"sync"
)

var (
	_ gentity.Application = (*GateServer)(nil)
	// singleton
	_gateServer *GateServer
)

type GateServer struct {
	*BaseServer
	config         *GateServerConfig
	clientListener Listener
	// WebSocket测试
	wsClientListener Listener
	clientsMutex     sync.RWMutex
	clients          map[int64]*network.ClientData
}

// gate服配置
type GateServerConfig struct {
	BaseServerConfig
	// WebSocket测试
	WsClientListenAddr     string
	WsClientListenerConfig ListenerConfig
}

func NewGateServer(ctx context.Context, configFile string) *GateServer {
	s := &GateServer{
		BaseServer: NewBaseServer(ctx, ServerType_Gate, configFile),
		config:     new(GateServerConfig),
	}
	s.readConfig()
	return s
}

// 初始化
func (this *GateServer) Init(ctx context.Context, configFile string) bool {
	_gateServer = this
	if !this.BaseServer.Init(ctx, configFile) {
		return false
	}
	this.clients = make(map[int64]*network.ClientData)
	this.initCache()
	this.initNetwork()
	logger.Info("GateServer.Init")
	return true
}

// 读取配置文件
func (this *GateServer) readConfig() {
	fileData, err := os.ReadFile(this.GetConfigFile())
	if err != nil {
		panic("read config file err")
	}
	this.config = new(GateServerConfig)
	err = json.Unmarshal(fileData, this.config)
	if err != nil {
		panic("decode config file err")
	}
	logger.Debug("%v", this.config)
	this.BaseServer.GetServerInfo().ServerId = this.config.ServerId
	this.BaseServer.GetServerInfo().ClientListenAddr = this.config.ClientListenAddr
}

// 初始化redis缓存
func (this *GateServer) initCache() {
	cache.NewRedis(this.config.RedisUri, this.config.RedisUsername, this.config.RedisPassword, this.config.RedisCluster)
	pong, err := cache.GetRedis().Ping(context.Background()).Result()
	if err != nil || pong == "" {
		panic("redis connect error")
	}
}

func (this *GateServer) initNetwork() {
	// 监听普通TCP客户端
	this.clientListener = network.ListenGateClient(this.config.ClientListenAddr, &ClientListerHandler{}, this.registerClientPacket)
	if this.clientListener == nil {
		panic("listen client failed")
	}

	// 监听WebSocket客户端
	if this.config.WsClientListenAddr != "" {
		// WebSocket测试
		this.wsClientListener = network.ListenWebSocketClient(this.config.WsClientListenAddr, &ClientListerHandler{}, this.registerClientPacket)
		if this.wsClientListener == nil {
			panic("listen websocket client failed")
		}
	}

	this.GetServerList().SetCache(cache.Get())
	// 连接其他服务器
	this.registerServerPacket(this.GetServerList().GetServerConnectionHandler())
	this.GetServerList().SetFetchAndConnectServerTypes(ServerType_Login, ServerType_Game)
}

// 注册客户端消息回调
func (this *GateServer) registerClientPacket(clientHandler *DefaultConnectionHandler) {
	// 手动注册消息回调
	clientHandler.Register(PacketCommand(pb.CmdClient_Cmd_AccountReg), this.routeToLoginServer, new(pb.AccountReg))
	clientHandler.Register(PacketCommand(pb.CmdClient_Cmd_LoginReq), this.routeToLoginServer, new(pb.LoginReq))
	clientHandler.Register(PacketCommand(pb.CmdClient_Cmd_PlayerEntryGameReq), this.routeToGameServerWithConnId, new(pb.PlayerEntryGameReq))
	clientHandler.Register(PacketCommand(pb.CmdClient_Cmd_CreatePlayerReq), this.routeToGameServerWithConnId, new(pb.CreatePlayerReq))
	clientHandler.SetUnRegisterHandler(this.routeToGameServer)
}

//// 客户端绑定连接
//func onBindConnReq(connection Connection, packet Packet) {
//	//req := packet.Message().(*pb.BindConnReq)
//}

// client -> gate -> loginServer
func (this *GateServer) routeToLoginServer(connection Connection, packet Packet) {
	message := packet.Message()
	data := packet.GetStreamData()
	var gatePacket *network.GatePacket
	if message != nil {
		gatePacket = network.NewGatePacket(0, packet.Command(), message)
	} else {
		gatePacket = network.NewGatePacketWithData(0, packet.Command(), data)
	}
	loginServers := this.GetServerList().GetServersByType(ServerType_Login)
	if len(loginServers) == 0 {
		logger.Debug("routeToLoginServerErr clientConn:%v cmd:%v noLoginServer", connection.GetConnectionId(), packet.Command())
		return
	}
	// 负载均衡:随机一个LoginServer
	randomServer := loginServers[rand.Intn(len(loginServers))]
	loginServerConn := this.GetServerList().GetServerConnection(randomServer.GetServerId())
	if loginServerConn == nil {
		logger.Debug("routeToLoginServerErr clientConn:%v cmd:%v serverId:%v", connection.GetConnectionId(), packet.Command(), randomServer.GetServerId())
		return
	}
	// 登录消息,附加上客户端的connId,转发给LoginServer
	gatePacket.SetPlayerId(int64(connection.GetConnectionId()))
	loginServerConn.SendPacket(gatePacket)
	logger.Debug("routeToLoginServer clientConn:%v cmd:%v serverId:%v", connection.GetConnectionId(), packet.Command(), randomServer.GetServerId())
}

// 登录期间,playerId还没确定,这时候GatePacket.PlayerId用来存储connId
func (this *GateServer) routeToGameServerWithConnId(connection Connection, packet Packet) {
	if clientData, ok := connection.GetTag().(*network.ClientData); ok {
		message := packet.Message()
		data := packet.GetStreamData()
		var gatePacket *network.GatePacket
		if message != nil {
			gatePacket = network.NewGatePacket(0, packet.Command(), message)
		} else {
			gatePacket = network.NewGatePacketWithData(0, packet.Command(), data)
		}
		// 附加上connId
		gatePacket.SetPlayerId(int64(clientData.ConnId))
		this.GetServerList().SendPacket(clientData.GameServerId, gatePacket)
		logger.Debug("routeToGameServerWithConnId clientConn:%v connId:%v cmd:%v serverId:%v", connection.GetConnectionId(),
			clientData.ConnId, packet.Command(), clientData.GameServerId)
	}
}

func (this *GateServer) routeToGameServer(connection Connection, packet Packet) {
	if clientData, ok := connection.GetTag().(*network.ClientData); ok {
		// 已验证过的客户端,转发给对应的GameServer
		message := packet.Message()
		data := packet.GetStreamData()
		var gatePacket *network.GatePacket
		if message != nil {
			gatePacket = network.NewGatePacket(0, packet.Command(), message)
		} else {
			gatePacket = network.NewGatePacketWithData(0, packet.Command(), data)
		}
		// 附加上playerId
		gatePacket.SetPlayerId(clientData.PlayerId)
		this.GetServerList().SendPacket(clientData.GameServerId, gatePacket)
		logger.Debug("routeToGameServer clientConn:%v playerId:%v cmd:%v serverId:%v", connection.GetConnectionId(),
			clientData.PlayerId, packet.Command(), clientData.GameServerId)
	}
}

// 注册服务器消息回调
func (this *GateServer) registerServerPacket(serverHandler *DefaultConnectionHandler) {
	serverHandler.Register(PacketCommand(pb.CmdClient_Cmd_AccountRes), this.routeToClientWithConnId, new(pb.AccountRes))
	serverHandler.Register(PacketCommand(pb.CmdClient_Cmd_LoginRes), this.onLoginRes, new(pb.LoginRes))
	serverHandler.Register(PacketCommand(pb.CmdClient_Cmd_CreatePlayerRes), this.routeToClientWithConnId, new(pb.CreatePlayerRes))
	serverHandler.Register(PacketCommand(pb.CmdClient_Cmd_PlayerEntryGameRes), this.onPlayerEntryGameRes, new(pb.PlayerEntryGameRes))
	serverHandler.SetUnRegisterHandler(this.routeToClient)
}

// 登录期间,playerId还没确定,这时候GatePacket.PlayerId用来存储connId
func (this *GateServer) routeToClientWithConnId(connection Connection, packet Packet) {
	gatePacket, _ := packet.(*network.GatePacket)
	clientConn := this.getClientConnectionByConnId(uint32(gatePacket.PlayerId()))
	if clientConn == nil {
		return
	}
	clientConn.SendPacket(NewProtoPacketEx(packet.Command(), packet.Message(), packet.GetStreamData()))
}

func (this *GateServer) onLoginRes(connection Connection, packet Packet) {
	res := packet.Message().(*pb.LoginRes)
	gatePacket, _ := packet.(*network.GatePacket)
	clientConnId := uint32(gatePacket.PlayerId())
	clientConn := this.getClientConnectionByConnId(clientConnId)
	if clientConn == nil {
		logger.Debug("onLoginRes clientConnNil connId:%v account:%v err:%v", clientConnId,
			res.AccountName, res.Error)
		return
	}
	if res.Error == "" {
		// 客户端登录成功,为该客户端连接设置绑定信息
		clientData := &network.ClientData{
			ConnId:       clientConn.GetConnectionId(),
			AccountId:    res.AccountId,
			GameServerId: res.GetGameServer().GetServerId(),
		}
		clientConn.SetTag(clientData)
	}
	clientConn.Send(packet.Command(), packet.Message())
	logger.Debug("onLoginRes connId:%v account:%v accountId:%v err:%v GameServerId:%v", clientConn.GetConnectionId(),
		res.AccountName, res.AccountId, res.Error, res.GetGameServer().GetServerId())
}

func (this *GateServer) onPlayerEntryGameRes(connection Connection, packet Packet) {
	res := packet.Message().(*pb.PlayerEntryGameRes)
	gatePacket, _ := packet.(*network.GatePacket)
	clientConnId := uint32(gatePacket.PlayerId())
	clientConn := this.getClientConnectionByConnId(clientConnId)
	if clientConn == nil {
		logger.Debug("onPlayerEntryGameRes clientConnNil connId:%v accountId:%v err:%v", clientConnId,
			res.AccountId, res.Error)
		return
	}
	if res.Error == "" {
		if clientData, ok := clientConn.GetTag().(*network.ClientData); ok {
			// 登录游戏服成功后,绑定客户端连接和playerId,后续的消息都可以用playerId来关联
			clientData.PlayerId = res.PlayerId
			this.clientsMutex.Lock()
			this.clients[clientData.PlayerId] = clientData
			this.clientsMutex.Unlock()
			logger.Debug("bindPlayerId connId:%v playerId:%v", clientConn.GetConnectionId(), res.PlayerId)
		}
	}
	clientConn.Send(packet.Command(), packet.Message())
	logger.Debug("onPlayerEntryGameRes connId:%v playerId:%v err:%v", clientConn.GetConnectionId(),
		res.PlayerId, res.Error)
}

func (this *GateServer) routeToClient(connection Connection, packet Packet) {
	gatePacket, _ := packet.(*network.GatePacket)
	this.clientsMutex.RLock()
	defer this.clientsMutex.RUnlock()
	if clientData, ok := this.clients[gatePacket.PlayerId()]; ok {
		clientConn := this.getClientConnectionByConnId(clientData.ConnId)
		if clientConn == nil {
			logger.Debug("routeToClientErr clientConn:%v playerId:%v cmd:%v", clientData.ConnId,
				clientData.PlayerId, packet.Command())
			return
		}
		clientConn.SendPacket(NewProtoPacketEx(packet.Command(), packet.Message(), packet.GetStreamData()))
		logger.Debug("routeToClient clientConn:%v playerId:%v cmd:%v message:%v dataLen:%v", clientData.ConnId,
			clientData.PlayerId, packet.Command(), packet.Message(), len(packet.GetStreamData()))
		return
	}
	logger.Debug("routeToClientErr playerId:%v packet:%v", gatePacket.PlayerId(), packet)
}

func (this *GateServer) getClientConnectionByConnId(clientConnId uint32) Connection {
	// Tcp和WebSocket的客户端分别由各自的Listener管理,但是ConnectionId是唯一的
	// 所以这里分别从不同的Listener查找
	// 当然,实际项目一般不会同时出现Tcp和WebSocket共存的情况
	clientConn := this.clientListener.GetConnection(clientConnId)
	if clientConn == nil && this.wsClientListener != nil {
		clientConn = this.wsClientListener.GetConnection(clientConnId)
	}
	return clientConn
}
