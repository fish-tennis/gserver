package gateserver

import (
	"context"
	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
	"gopkg.in/yaml.v3"
	"log/slog"
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
	clients          map[int64]*network.ClientData // key: playerId
}

// gate服配置
type GateServerConfig struct {
	BaseServerConfig `yaml:",inline"`
	// WebSocket测试
	WsClient ListerConfig `yaml:"WsClient"`
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
func (s *GateServer) Init(ctx context.Context, configFile string) bool {
	_gateServer = s
	if !s.BaseServer.Init(ctx, configFile) {
		return false
	}
	s.clients = make(map[int64]*network.ClientData)
	s.initCache()
	s.initNetwork()
	logger.Info("GateServer.Init")
	return true
}

// 读取配置文件
func (s *GateServer) readConfig() {
	fileData, err := os.ReadFile(s.GetConfigFile())
	if err != nil {
		panic("read config file err")
	}
	s.config = &GateServerConfig{}
	err = yaml.Unmarshal(fileData, s.config)
	if err != nil {
		panic("decode config file err")
	}
	logger.Debug("%v", s.config)
	s.BaseServer.GetServerInfo().ServerId = s.config.ServerId
	s.BaseServer.GetServerInfo().ClientListenAddr = s.config.Client.Addr
}

// 初始化redis缓存
func (s *GateServer) initCache() {
	cache.NewRedis(s.config.Redis.Uri, s.config.Redis.UserName, s.config.Redis.Password, s.config.Redis.Cluster)
	pong, err := cache.GetRedis().Ping(context.Background()).Result()
	if err != nil || pong == "" {
		panic("redis connect error")
	}
}

func (s *GateServer) initNetwork() {
	// 监听普通TCP客户端
	s.clientListener = network.ListenGateClient(s.config.Client.Addr, &ClientListerHandler{}, s.registerClientPacket)
	if s.clientListener == nil {
		panic("listen client failed")
	}

	// 监听WebSocket客户端
	if s.config.WsClient.Addr != "" {
		// WebSocket测试
		s.wsClientListener = network.ListenWebSocketClient(s.config.WsClient.Addr, &ClientListerHandler{}, s.registerClientPacket)
		if s.wsClientListener == nil {
			panic("listen websocket client failed")
		}
	}

	s.GetServerList().SetCache(cache.Get())
	// 连接其他服务器
	s.registerServerPacket(s.GetServerList().GetServerConnectionHandler())
	s.GetServerList().SetFetchAndConnectServerTypes(ServerType_Login, ServerType_Game)
}

// 注册客户端消息回调
func (s *GateServer) registerClientPacket(clientHandler *DefaultConnectionHandler) {
	// 手动注册消息回调
	clientHandler.Register(PacketCommand(pb.CmdClient_Cmd_AccountReg), s.routeToLoginServer, new(pb.AccountReg))
	clientHandler.Register(PacketCommand(pb.CmdClient_Cmd_LoginReq), s.routeToLoginServer, new(pb.LoginReq))
	clientHandler.Register(PacketCommand(pb.CmdClient_Cmd_PlayerEntryGameReq), s.routeToGameServerWithConnId, new(pb.PlayerEntryGameReq))
	clientHandler.Register(PacketCommand(pb.CmdClient_Cmd_CreatePlayerReq), s.routeToGameServerWithConnId, new(pb.CreatePlayerReq))
	clientHandler.SetUnRegisterHandler(s.routeToGameServer)
}

//// 客户端绑定连接
//func onBindConnReq(connection Connection, packet Packet) {
//	//req := packet.Message().(*pb.BindConnReq)
//}

// client -> gateserver -> loginServer
func (s *GateServer) routeToLoginServer(connection Connection, packet Packet) {
	message := packet.Message()
	data := packet.GetStreamData()
	var gatePacket *network.GatePacket
	if message != nil {
		gatePacket = network.NewGatePacket(0, packet.Command(), message)
	} else {
		gatePacket = network.NewGatePacketWithData(0, packet.Command(), data)
	}
	loginServers := s.GetServerList().GetServersByType(ServerType_Login)
	if len(loginServers) == 0 {
		logger.Debug("routeToLoginServerErr clientConn:%v cmd:%v noLoginServer", connection.GetConnectionId(), packet.Command())
		return
	}
	// 负载均衡:随机一个LoginServer
	randomServer := loginServers[rand.Intn(len(loginServers))]
	loginServerConn := s.GetServerList().GetServerConnection(randomServer.GetServerId())
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
func (s *GateServer) routeToGameServerWithConnId(connection Connection, packet Packet) {
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
		s.GetServerList().SendPacket(clientData.GameServerId, gatePacket)
		logger.Debug("routeToGameServerWithConnId clientConn:%v connId:%v cmd:%v serverId:%v", connection.GetConnectionId(),
			clientData.ConnId, packet.Command(), clientData.GameServerId)
	}
}

func (s *GateServer) routeToGameServer(connection Connection, packet Packet) {
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
		if !s.GetServerList().SendPacket(clientData.GameServerId, gatePacket) {
			connection.Send(PacketCommand(pb.CmdClient_Cmd_ErrorRes), &pb.ErrorRes{
				Command:   int32(packet.Command()),
				ResultStr: "GameServerNotReached",
			})
			return
		}
		logger.Debug("routeToGameServer clientConn:%v playerId:%v cmd:%v serverId:%v", connection.GetConnectionId(),
			clientData.PlayerId, packet.Command(), clientData.GameServerId)
	}
}

// 注册服务器消息回调
func (s *GateServer) registerServerPacket(serverHandler *DefaultConnectionHandler) {
	serverHandler.Register(PacketCommand(pb.CmdClient_Cmd_AccountRes), s.routeToClientWithConnId, new(pb.AccountRes))
	serverHandler.Register(PacketCommand(pb.CmdClient_Cmd_LoginRes), s.onLoginRes, new(pb.LoginRes))
	serverHandler.Register(PacketCommand(pb.CmdClient_Cmd_CreatePlayerRes), s.routeToClientWithConnId, new(pb.CreatePlayerRes))
	serverHandler.Register(PacketCommand(pb.CmdClient_Cmd_PlayerEntryGameRes), s.onPlayerEntryGameRes, new(pb.PlayerEntryGameRes))
	serverHandler.SetUnRegisterHandler(s.routeToClient)
}

// 登录期间,playerId还没确定,这时候GatePacket.PlayerId用来存储connId
func (s *GateServer) routeToClientWithConnId(connection Connection, packet Packet) {
	gatePacket, _ := packet.(*network.GatePacket)
	clientConn := s.getClientConnectionByConnId(uint32(gatePacket.PlayerId()))
	if clientConn == nil {
		return
	}
	clientConn.SendPacket(NewProtoPacketEx(packet.Command(), packet.Message(), packet.GetStreamData()))
}

func (s *GateServer) onLoginRes(connection Connection, packet Packet) {
	res := packet.Message().(*pb.LoginRes)
	gatePacket, _ := packet.(*network.GatePacket)
	clientConnId := uint32(gatePacket.PlayerId())
	clientConn := s.getClientConnectionByConnId(clientConnId)
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

func (s *GateServer) onPlayerEntryGameRes(connection Connection, packet Packet) {
	res := packet.Message().(*pb.PlayerEntryGameRes)
	gatePacket, _ := packet.(*network.GatePacket)
	clientConnId := uint32(gatePacket.PlayerId())
	clientConn := s.getClientConnectionByConnId(clientConnId)
	if clientConn == nil {
		logger.Debug("onPlayerEntryGameRes clientConnNil connId:%v accountId:%v err:%v", clientConnId,
			res.AccountId, res.Error)
		return
	}
	if res.Error == "" {
		if clientData, ok := clientConn.GetTag().(*network.ClientData); ok {
			// 登录游戏服成功后,绑定客户端连接和playerId,后续的消息都可以用playerId来关联
			clientData.PlayerId = res.PlayerId
			s.clientsMutex.Lock()
			s.clients[clientData.PlayerId] = clientData
			s.clientsMutex.Unlock()
			logger.Debug("bindPlayerId connId:%v playerId:%v", clientConn.GetConnectionId(), res.PlayerId)
		}
	}
	clientConn.Send(packet.Command(), packet.Message())
	logger.Debug("onPlayerEntryGameRes connId:%v playerId:%v err:%v", clientConn.GetConnectionId(),
		res.PlayerId, res.Error)
}

func (s *GateServer) routeToClient(connection Connection, packet Packet) {
	gatePacket, _ := packet.(*network.GatePacket)
	s.clientsMutex.RLock()
	clientData, ok := s.clients[gatePacket.PlayerId()]
	defer s.clientsMutex.RUnlock()
	if ok {
		clientConn := s.getClientConnectionByConnId(clientData.ConnId)
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

func (s *GateServer) getClientConnectionByConnId(clientConnId uint32) Connection {
	// Tcp和WebSocket的客户端分别由各自的Listener管理,但是ConnectionId是唯一的
	// 所以这里分别从不同的Listener查找
	// 当然,实际项目一般不会同时出现Tcp和WebSocket共存的情况
	clientConn := s.clientListener.GetConnection(clientConnId)
	if clientConn == nil && s.wsClientListener != nil {
		clientConn = s.wsClientListener.GetConnection(clientConnId)
	}
	return clientConn
}

// 网关转发客户端消息到其他服务器,发生错误
func (s *GateServer) onGateRouteClientPacketError(connection Connection, packet Packet) {
	gatePacket, _ := packet.(*network.GatePacket)
	s.clientsMutex.RLock()
	clientData, ok := s.clients[gatePacket.PlayerId()]
	defer s.clientsMutex.RUnlock()
	if ok {
		slog.Debug("onGateRouteClientPacketError", "clientData", clientData)
		clientData.GameServerId = 0
		clientConn := s.getClientConnectionByConnId(clientData.ConnId)
		if clientConn == nil {
			logger.Debug("onGateRouteClientPacketError", "ConnId", clientData.ConnId)
			return
		}
		clientConn.SendPacket(NewProtoPacketEx(packet.Command(), packet.Message(), packet.GetStreamData()))
	}
}
