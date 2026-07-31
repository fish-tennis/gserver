package gateserver

import (
	"context"
	"fmt"
	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
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

func NewGateServer(ctx context.Context, configFile string, cfgDir string) *GateServer {
	s := &GateServer{
		BaseServer: NewBaseServer(ctx, ServerType_Gate, configFile, cfgDir),
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
		panic(fmt.Sprintf("read %v err:%v", s.GetConfigFile(), err))
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
		slog.Error("redis connect error", "uri", s.config.Redis.Uri, "cluster", s.config.Redis.Cluster, "err", err)
		panic(fmt.Sprintf("redis connect error: uri:%v(%v) err:%v", s.config.Redis.Uri, s.config.Redis.Cluster, err))
	}
}

func (s *GateServer) initNetwork() {
	// 监听普通TCP客户端
	s.clientListener = network.ListenGateClient(s.config.Client.Addr, &ClientListerHandler{}, s.registerClientPacket)
	if s.clientListener == nil {
		panic("listen client failed")
	}
	slog.Info("listen client", "addr", s.config.Client.Addr)

	// 监听WebSocket客户端
	if s.config.WsClient.Addr != "" {
		// WebSocket测试
		s.wsClientListener = network.ListenWebSocketClient(s.config.WsClient.Addr, &ClientListerHandler{}, s.registerClientPacket)
		if s.wsClientListener == nil {
			panic("listen websocket client failed")
		}
		slog.Info("listen websocket client", "addr", s.config.WsClient.Addr)
	}

	s.GetServerList().SetCache(cache.Get())
	// 连接其他服务器
	s.registerServerPacket(s.GetServerList().GetServerConnectionHandler())
	s.GetServerList().SetFetchAndConnectServerTypes(ServerType_Login, ServerType_Game)
}

// 注册客户端消息回调
func (s *GateServer) registerClientPacket(clientHandler *DefaultConnectionHandler) {
	// 手动注册消息回调
	network.RegisterPacketHandler(clientHandler, new(pb.AccountReg), s.routeToLoginServer)
	network.RegisterPacketHandler(clientHandler, new(pb.LoginReq), s.routeToLoginServer)
	network.RegisterPacketHandler(clientHandler, new(pb.PlayerEntryGameReq), s.routeToGameServerWithConnId)
	network.RegisterPacketHandler(clientHandler, new(pb.CreatePlayerReq), s.routeToGameServerWithConnId)
	// 重连请求:客户端是新连接没有GameServerId,需通过Redis查找玩家所在游戏服来路由
	network.RegisterPacketHandler(clientHandler, new(pb.PlayerReconnectGameReq), s.routeReconnectToGameServer)

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

// routeReconnectToGameServer 重连请求路由
// 客户端重连时是新连接,没有ClientData,由请求体中的GameServerId指定目标游戏服
// 路由策略:
//   - 玩家在线(Redis有记录):校验req.GameServerId与在线游戏服一致,不一致则通知重新登录
//   - 玩家不在线(超过保留期/从未在线):直接转发到req.GameServerId,由游戏服从数据库加载
// client -> gateserver -> gameServer
func (s *GateServer) routeReconnectToGameServer(connection Connection, packet Packet) {
	req := packet.Message().(*pb.PlayerReconnectGameReq)
	targetGameServerId := req.GetGameServerId()
	// 客户端未携带有效GameServerId,通知重新登录
	if targetGameServerId <= 0 {
		logger.Debug("routeReconnectToGameServer invalidGameServerId playerId:%v", req.GetPlayerId())
		cmd := network.GetCommandByProto(new(pb.ErrorRes))
		connection.Send(PacketCommand(cmd), &pb.ErrorRes{
			Command:  int32(packet.Command()),
			ResultId: int32(pb.ErrorCode_ErrorCode_ReconnectNeedRelogin),
		})
		return
	}
	// 玩家在线时校验GameServerId一致性,防止客户端用过期的GameServerId重连到错误的服务器
	_, onlineGameServerId := cache.GetOnlinePlayer(req.GetPlayerId())
	if onlineGameServerId > 0 && onlineGameServerId != targetGameServerId {
		logger.Debug("routeReconnectToGameServer gameServerIdMismatch playerId:%v online:%v req:%v",
			req.GetPlayerId(), onlineGameServerId, targetGameServerId)
		cmd := network.GetCommandByProto(new(pb.ErrorRes))
		connection.Send(PacketCommand(cmd), &pb.ErrorRes{
			Command:  int32(packet.Command()),
			ResultId: int32(pb.ErrorCode_ErrorCode_ReconnectNeedRelogin),
		})
		return
	}
	// 玩家在线且GameServerId一致,或玩家不在线(转发到req.GameServerId由游戏服加载DB)
	message := packet.Message()
	data := packet.GetStreamData()
	var gatePacket *network.GatePacket
	if message != nil {
		gatePacket = network.NewGatePacket(0, packet.Command(), message)
	} else {
		gatePacket = network.NewGatePacketWithData(0, packet.Command(), data)
	}
	// 登录期间playerId还没确定,用connId临时存储在GatePacket.PlayerId中
	gatePacket.SetPlayerId(int64(connection.GetConnectionId()))
	s.GetServerList().SendPacket(targetGameServerId, gatePacket)
	logger.Debug("routeReconnectToGameServer clientConn:%v playerId:%v cmd:%v serverId:%v online:%v",
		connection.GetConnectionId(), req.GetPlayerId(), packet.Command(), targetGameServerId, onlineGameServerId)
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
			cmd := network.GetCommandByProto(new(pb.ErrorRes))
			connection.Send(PacketCommand(cmd), &pb.ErrorRes{
				Command:   int32(packet.Command()),
				ResultStr: "GameServerNotReached",
			})
			return
		}
		logger.Debug("routeToGameServer clientConn:%v playerId:%v cmd:%v serverId:%v message:%v", connection.GetConnectionId(),
			clientData.PlayerId, packet.Command(), clientData.GameServerId, proto.MessageName(message))
	}
}

// 注册服务器消息回调
func (s *GateServer) registerServerPacket(serverHandler *DefaultConnectionHandler) {
	network.RegisterPacketHandler(serverHandler, new(pb.AccountRes), s.routeToClientWithConnId)
	network.RegisterPacketHandler(serverHandler, new(pb.LoginRes), s.onLoginRes)
	network.RegisterPacketHandler(serverHandler, new(pb.CreatePlayerRes), s.routeToClientWithConnId)
	network.RegisterPacketHandler(serverHandler, new(pb.PlayerEntryGameRes), s.onPlayerEntryGameRes)
	// 重连响应:需要为新的客户端连接建立ClientData绑定
	network.RegisterPacketHandler(serverHandler, new(pb.PlayerReconnectGameRes), s.onPlayerReconnectGameRes)
	network.RegisterPacketHandler(serverHandler, new(pb.GateRouteClientPacketError), s.onGateRouteClientPacketError)

	serverHandler.SetUnRegisterHandler(s.routeToClient)
}

// 登录期间,playerId还没确定,这时候GatePacket.PlayerId用来存储connId
func (s *GateServer) routeToClientWithConnId(connection Connection, packet Packet) {
	gatePacket, _ := packet.(*network.GatePacket)
	clientConn := s.getClientConnectionByConnId(uint32(gatePacket.PlayerId()))
	if clientConn == nil {
		return
	}
	clientConn.SendPacket(NewProtoPacketEx(packet.Command(), packet.Message(), packet.GetStreamData()).SetErrorCode(packet.ErrorCode()))
}

func (s *GateServer) onLoginRes(connection Connection, packet Packet) {
	res := packet.Message().(*pb.LoginRes)
	gatePacket, _ := packet.(*network.GatePacket)
	clientConnId := uint32(gatePacket.PlayerId())
	clientConn := s.getClientConnectionByConnId(clientConnId)
	if clientConn == nil {
		logger.Debug("onLoginRes clientConnNil connId:%v account:%v err:%v", clientConnId,
			res.AccountName, packet.ErrorCode())
		return
	}
	if packet.ErrorCode() == 0 {
		// 客户端登录成功,为该客户端连接设置绑定信息
		clientData := &network.ClientData{
			ConnId:       clientConn.GetConnectionId(),
			AccountId:    res.AccountId,
			GameServerId: res.GetGameServer().GetServerId(),
		}
		clientConn.SetTag(clientData)
	}
	clientPacket := NewProtoPacket(packet.Command(), packet.Message()).SetErrorCode(packet.ErrorCode())
	clientConn.SendPacket(clientPacket)
	logger.Debug("onLoginRes connId:%v account:%v accountId:%v err:%v GameServerId:%v", clientConn.GetConnectionId(),
		res.AccountName, res.AccountId, packet.ErrorCode(), res.GetGameServer().GetServerId())
}

func (s *GateServer) onPlayerEntryGameRes(connection Connection, packet Packet) {
	res := packet.Message().(*pb.PlayerEntryGameRes)
	gatePacket, _ := packet.(*network.GatePacket)
	clientConnId := uint32(gatePacket.PlayerId())
	clientConn := s.getClientConnectionByConnId(clientConnId)
	if clientConn == nil {
		logger.Debug("onPlayerEntryGameRes clientConnNil connId:%v accountId:%v err:%v", clientConnId,
			res.AccountId, packet.ErrorCode())
		return
	}
	if packet.ErrorCode() == 0 {
		if clientData, ok := clientConn.GetTag().(*network.ClientData); ok {
			// 登录游戏服成功后,绑定客户端连接和playerId,后续的消息都可以用playerId来关联
			// 在写锁保护下修改 clientData.PlayerId,避免跨协程数据竞争
			s.clientsMutex.Lock()
			clientData.PlayerId = res.PlayerId
			s.clients[clientData.PlayerId] = clientData
			s.clientsMutex.Unlock()
			logger.Debug("bindPlayerId connId:%v playerId:%v", clientConn.GetConnectionId(), res.PlayerId)
		}
	}
	clientPacket := NewProtoPacket(packet.Command(), packet.Message()).SetErrorCode(packet.ErrorCode())
	clientConn.SendPacket(clientPacket)
	logger.Debug("onPlayerEntryGameRes connId:%v playerId:%v err:%v", clientConn.GetConnectionId(),
		res.PlayerId, packet.ErrorCode())
}

// onPlayerReconnectGameRes 处理游戏服返回的重连响应
// 重连成功时,客户端是全新连接,需要为其建立ClientData绑定(包含GameServerId)
func (s *GateServer) onPlayerReconnectGameRes(connection Connection, packet Packet) {
	res := packet.Message().(*pb.PlayerReconnectGameRes)
	gatePacket, _ := packet.(*network.GatePacket)
	clientConnId := uint32(gatePacket.PlayerId())
	clientConn := s.getClientConnectionByConnId(clientConnId)
	if clientConn == nil {
		logger.Debug("onPlayerReconnectGameRes clientConnNil connId:%v playerId:%v err:%v", clientConnId,
			res.PlayerId, packet.ErrorCode())
		return
	}
	if packet.ErrorCode() == 0 {
		// 重连成功:为新连接创建ClientData并绑定,恢复s.clients映射
		// 后续的业务消息才能通过routeToGameServer路由、GameServer下发消息才能通过routeToClient找到客户端
		clientData := &network.ClientData{
			ConnId:       clientConn.GetConnectionId(),
			AccountId:    res.AccountId,
			PlayerId:     res.PlayerId,
			GameServerId: res.GameServerId,
		}
		s.clientsMutex.Lock()
		// 先清理可能残留的旧连接绑定:旧连接可能因为网络"假死"还未触发OnConnectionDisconnect
		// 清除旧连接的tag后,旧连接延迟断开时OnConnectionDisconnect会直接return(GetTag()==nil)
		// 不会误删新连接的s.clients映射,也不会发送虚假的ClientDisconnect通知
		if oldClientData, ok := s.clients[res.PlayerId]; ok {
			if oldConn := s.getClientConnectionByConnId(oldClientData.ConnId); oldConn != nil && oldConn != clientConn {
				oldConn.SetTag(nil)
				logger.Debug("onPlayerReconnectGameRes clear old conn:%v playerId:%v",
					oldConn.GetConnectionId(), res.PlayerId)
			}
		}
		clientConn.SetTag(clientData)
		s.clients[clientData.PlayerId] = clientData
		s.clientsMutex.Unlock()
		logger.Debug("onPlayerReconnectGameRes bind connId:%v playerId:%v gameServerId:%v",
			clientConn.GetConnectionId(), res.PlayerId, res.GameServerId)
	}
	clientPacket := NewProtoPacket(packet.Command(), packet.Message()).SetErrorCode(packet.ErrorCode())
	clientConn.SendPacket(clientPacket)
	logger.Debug("onPlayerReconnectGameRes connId:%v playerId:%v err:%v", clientConn.GetConnectionId(),
		res.PlayerId, packet.ErrorCode())
}

func (s *GateServer) routeToClient(connection Connection, packet Packet) {
	gatePacket, _ := packet.(*network.GatePacket)
	// 持锁期间只拷贝必要数据,释放锁后再发送,避免慢客户端阻塞写锁
	s.clientsMutex.RLock()
	clientData, ok := s.clients[gatePacket.PlayerId()]
	connId := uint32(0)
	if ok {
		connId = clientData.ConnId
	}
	s.clientsMutex.RUnlock()
	if ok {
		clientConn := s.getClientConnectionByConnId(connId)
		if clientConn == nil {
			logger.Debug("routeToClientErr clientConn:%v playerId:%v cmd:%v", connId,
				gatePacket.PlayerId(), packet.Command())
			return
		}
		clientConn.SendPacket(NewProtoPacketEx(packet.Command(), packet.Message(), packet.GetStreamData()))
		logger.Debug("routeToClient clientConn:%v playerId:%v cmd:%v message:%v dataLen:%v", connId,
			gatePacket.PlayerId(), packet.Command(), packet.Message(), len(packet.GetStreamData()))
		return
	}
	logger.Debug("routeToClientErr playerId:%v cmd:%v message:%v packet:%v", gatePacket.PlayerId(), packet.Command(),
		proto.MessageName(packet.Message()), packet)
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

// 网关转发客户端消息到GameServer时发生错误(GameServer上找不到该玩家)
// 给客户端返回一个能识别的错误,而不是转发内部协议
func (s *GateServer) onGateRouteClientPacketError(connection Connection, packet Packet) {
	gatePacket, _ := packet.(*network.GatePacket)
	errorMsg := packet.Message().(*pb.GateRouteClientPacketError)
	s.clientsMutex.RLock()
	clientData, ok := s.clients[gatePacket.PlayerId()]
	connId := uint32(0)
	if ok {
		connId = clientData.ConnId
	}
	s.clientsMutex.RUnlock()
	if ok {
		logger.Debug("onGateRouteClientPacketError connId:%v playerId:%v cmd:%v reason:%v",
			connId, gatePacket.PlayerId(), errorMsg.GetCommand(), errorMsg.GetResultStr())
		clientConn := s.getClientConnectionByConnId(connId)
		if clientConn == nil {
			return
		}
		// 转换成客户端能识别的 ErrorRes
		cmd := network.GetCommandByProto(new(pb.ErrorRes))
		clientConn.Send(PacketCommand(cmd), &pb.ErrorRes{
			Command:   errorMsg.GetCommand(),
			ResultStr: errorMsg.GetResultStr(),
		})
	}
}
