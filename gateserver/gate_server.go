package gateserver

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"

	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
)

var (
	_ gentity.Application = (*GateServer)(nil)
	// singleton
	_gateServer *GateServer
)

type GateServer struct {
	*BaseServer
	clientListener Listener
	// WebSocket测试
	wsClientListener Listener
	clientsMutex     sync.RWMutex
	clients          map[int64]*network.ClientData // key: playerId
}

func NewGateServer(ctx context.Context, configFile string, cfgDir string) *GateServer {
	s := &GateServer{
		BaseServer: NewBaseServer(ctx, ServerType_Gate, configFile, cfgDir),
	}
	s.ReadConfig()
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
	slog.Info("GateServer.Init")
	return true
}

// 初始化redis缓存
func (s *GateServer) initCache() {
	cache.NewRedis(s.GetConfig().Redis.Uri, s.GetConfig().Redis.UserName, s.GetConfig().Redis.Password, s.GetConfig().Redis.Cluster, s.GetConfig().Redis.DB)
	pong, err := cache.GetRedis().Ping(context.Background()).Result()
	if err != nil || pong == "" {
		slog.Error("redis connect error", "uri", s.GetConfig().Redis.Uri, "cluster", s.GetConfig().Redis.Cluster, "err", err)
		panic(fmt.Sprintf("redis connect error: uri:%v(%v) err:%v", s.GetConfig().Redis.Uri, s.GetConfig().Redis.Cluster, err))
	}
}

func (s *GateServer) initNetwork() {
	// 监听普通TCP客户端
	if s.GetConfig().Client.Addr != "" {
		s.clientListener = network.ListenGateClient(s.GetConfig().Client.Addr, &ClientListerHandler{}, s.registerClientPacket)
		if s.clientListener == nil {
			panic("listen client failed")
		}
		slog.Info("listen tcp client", "addr", s.GetConfig().Client.Addr)
	}

	// 监听WebSocket客户端
	if s.GetConfig().WsClient.Addr != "" {
		// WebSocket测试
		s.wsClientListener = network.ListenWebSocketClient(s.GetConfig().WsClient.Addr, &ClientListerHandler{}, s.registerClientPacket)
		if s.wsClientListener == nil {
			panic("listen websocket client failed")
		}
		slog.Info("listen websocket client", "addr", s.GetConfig().WsClient.Addr)
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

// 转发失败时构造统一的 ErrorRes 回客户端:
//   - Command 用原请求命令号,让客户端能识别具体是哪个请求失败
//   - errorCode 同时写入 ErrorRes.ResultId 和 packet header,客户端可按任一方式判断失败
//   - rpcCallId 原样透传,保证 Req/Res 配对,避免客户端只能等 RPC 超时
//
// 之所以单独抽出来:多条转发失败路径都需要相同的回包逻辑,统一后既能避免遗漏
// rpcCallId/errorCode,也让客户端的错误处理保持一致
func (s *GateServer) sendRouteErrorRes(connection Connection, origCmd PacketCommand, rpcCallId uint32, errorCode pb.ErrorCode, reason string) {
	cmd := network.GetCommandByProto(new(pb.ErrorRes))
	errPacket := NewProtoPacketEx(PacketCommand(cmd), &pb.ErrorRes{
		Command:   int32(origCmd),
		ResultId:  int32(errorCode),
		ResultStr: reason,
	})
	errPacket.SetErrorCode(uint32(errorCode)).SetRpcCallId(rpcCallId)
	connection.SendPacket(errPacket)
}

// 检查服务器运行状态,非Running状态时拒绝请求
// 返回true表示已拒绝(调用方应直接return),false表示正常运行可继续处理
// 服务器关闭中(Exit)时回 ServerClosing,其他非运行状态(如Init)回 TryLater
func (s *GateServer) checkRunning(connection Connection, packet Packet) bool {
	if s.IsRunning() {
		return false
	}
	status := s.GetStatus()
	var errCode pb.ErrorCode
	var reason string
	if status == ServerStatus_Exit {
		errCode = pb.ErrorCode_ErrorCode_ServerClosing
		reason = "ServerClosing"
	} else {
		errCode = pb.ErrorCode_ErrorCode_TryLater
		reason = "ServerNotRunning"
	}
	s.sendRouteErrorRes(connection, packet.Command(), packet.RpcCallId(), errCode, reason)
	return true
}

// client -> gateserver -> loginServer
func (s *GateServer) routeToLoginServer(connection Connection, packet Packet) {
	if s.checkRunning(connection, packet) {
		return
	}
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
		slog.Debug("routeToLoginServerErr", "clientConn", connection.GetConnectionId(), "cmd", packet.Command(), "error", "noLoginServer")
		// 没有 LoginServer 可用,必须回错误响应,否则带 rpcCallId 的请求会让客户端一直等到超时
		s.sendRouteErrorRes(connection, packet.Command(), packet.RpcCallId(), pb.ErrorCode_ErrorCode_TryLater, "NoLoginServer")
		return
	}
	// 负载均衡:随机一个LoginServer
	randomServer := loginServers[rand.Intn(len(loginServers))]
	loginServerConn := s.GetServerList().GetServerConnection(randomServer.GetServerId())
	if loginServerConn == nil {
		slog.Debug("routeToLoginServerErr", "clientConn", connection.GetConnectionId(), "cmd", packet.Command(), "serverId", randomServer.GetServerId())
		// 选中的 LoginServer 连接不可用,同样需要回错误响应,避免客户端等超时
		s.sendRouteErrorRes(connection, packet.Command(), packet.RpcCallId(), pb.ErrorCode_ErrorCode_TryLater, "LoginServerNotReached")
		return
	}
	// 登录消息,附加上客户端的connId,转发给LoginServer
	gatePacket.SetPlayerId(int64(connection.GetConnectionId()))
	// 原样透传客户端携带的 rpcCallId,保证请求-响应往返一致
	gatePacket.SetRpcCallId(packet.RpcCallId())
	loginServerConn.SendPacket(gatePacket)
	slog.Debug("routeToLoginServer", "clientConn", connection.GetConnectionId(), "cmd", packet.Command(), "serverId", randomServer.GetServerId())
}

// 登录期间,playerId还没确定,这时候GatePacket.PlayerId用来存储connId
func (s *GateServer) routeToGameServerWithConnId(connection Connection, packet Packet) {
	if s.checkRunning(connection, packet) {
		return
	}
	clientData, ok := connection.GetTag().(*network.ClientData)
	if !ok {
		slog.Debug("routeToGameServerWithConnIdErr", "clientConn", connection.GetConnectionId(), "cmd", packet.Command(), "error", "clientDataNotBound")
		// clientData 未绑定意味着会话异常(连接未完成握手或已被清理),原实现静默 return 会导致客户端只能等 RPC 超时
		// 回 SessionError 让客户端立即触发重连流程
		s.sendRouteErrorRes(connection, packet.Command(), packet.RpcCallId(), pb.ErrorCode_ErrorCode_SessionError, "ClientDataNotBound")
		return
	}
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
	// 原样透传客户端携带的 rpcCallId
	gatePacket.SetRpcCallId(packet.RpcCallId())
	if !s.GetServerList().SendPacket(clientData.GetGameServerId(), gatePacket) {
		// GameServer 不可达时回错误响应,语义与 routeToGameServer 保持一致:
		// 登录期请求(如 PlayerEntryGameReq)同样带 rpcCallId,不回包会让客户端卡到超时
		s.sendRouteErrorRes(connection, packet.Command(), packet.RpcCallId(), pb.ErrorCode_ErrorCode_TryLater, "GameServerNotReached")
		return
	}
	slog.Debug("routeToGameServerWithConnId", "clientConn", connection.GetConnectionId(), "connId", clientData.ConnId, "cmd", packet.Command(), "serverId", clientData.GetGameServerId())
}

// routeReconnectToGameServer 重连请求路由
// 客户端重连时是新连接,没有ClientData,由请求体中的GameServerId指定目标游戏服
// 路由策略:
//   - 玩家在线(Redis有记录):校验req.GameServerId与在线游戏服一致,不一致则通知重新登录
//   - 玩家不在线(超过保留期/从未在线):直接转发到req.GameServerId,由游戏服从数据库加载
//
// client -> gateserver -> gameServer
func (s *GateServer) routeReconnectToGameServer(connection Connection, packet Packet) {
	if s.checkRunning(connection, packet) {
		return
	}
	req := packet.Message().(*pb.PlayerReconnectGameReq)
	targetGameServerId := req.GetGameServerId()
	// 客户端未携带有效GameServerId,通知重新登录
	if targetGameServerId <= 0 {
		slog.Debug("routeReconnectToGameServer invalidGameServerId", "playerId", req.GetPlayerId())
		s.sendRouteErrorRes(connection, packet.Command(), packet.RpcCallId(),
			pb.ErrorCode_ErrorCode_ReconnectNeedRelogin, "InvalidGameServerId")
		return
	}
	// 玩家在线时校验GameServerId一致性,防止客户端用过期的GameServerId重连到错误的服务器
	_, onlineGameServerId := cache.GetOnlinePlayer(req.GetPlayerId())
	if onlineGameServerId > 0 && onlineGameServerId != targetGameServerId {
		slog.Debug("routeReconnectToGameServer gameServerIdMismatch", "playerId", req.GetPlayerId(), "onlineGameServerId", onlineGameServerId, "targetGameServerId", targetGameServerId)
		s.sendRouteErrorRes(connection, packet.Command(), packet.RpcCallId(),
			pb.ErrorCode_ErrorCode_ReconnectNeedRelogin, "GameServerIdMismatch")
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
	// 原样透传客户端携带的 rpcCallId,保证重连的请求-响应配对
	gatePacket.SetRpcCallId(packet.RpcCallId())
	if !s.GetServerList().SendPacket(targetGameServerId, gatePacket) {
		s.sendRouteErrorRes(connection, packet.Command(), packet.RpcCallId(),
			pb.ErrorCode_ErrorCode_TryLater, "GameServerNotReached")
		return
	}
	slog.Debug("routeReconnectToGameServer", "clientConn", connection.GetConnectionId(), "playerId", req.GetPlayerId(), "cmd", packet.Command(), "serverId", targetGameServerId, "onlineGameServerId", onlineGameServerId)
}

func (s *GateServer) routeToGameServer(connection Connection, packet Packet) {
	if s.checkRunning(connection, packet) {
		return
	}
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
		// PlayerId 和 GameServerId 均用 atomic 读,保证跨协程可见性
		playerId := clientData.GetPlayerId()
		gameServerId := clientData.GetGameServerId()
		// 附加上playerId
		gatePacket.SetPlayerId(playerId)
		// 原样透传客户端携带的 rpcCallId
		gatePacket.SetRpcCallId(packet.RpcCallId())
		if !s.GetServerList().SendPacket(gameServerId, gatePacket) {
			// GameServer 不可达时回错误响应,统一走 sendRouteErrorRes:
			// 既透传 rpcCallId 保证 Req/Res 配对,又带上 errorCode 让客户端能按 header 判断失败
			s.sendRouteErrorRes(connection, packet.Command(), packet.RpcCallId(), pb.ErrorCode_ErrorCode_TryLater, "GameServerNotReached")
			return
		}
		slog.Debug("routeToGameServer", "clientConn", connection.GetConnectionId(), "playerId", clientData.GetPlayerId(), "cmd", packet.Command(), "serverId", clientData.GetGameServerId(), "message", proto.MessageName(message))
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
	// GameServer 发现玩家不在本服(跨服迁移/掉线重连等)时,会回 GateRouteClientPacketError。
	// 必须注册专门 handler:否则该消息会走默认 routeToClient 被原样转发给客户端,
	// 客户端收到的命令号是 GateRouteClientPacketError 的消息号而非原请求号,无法识别是哪个请求失败
	network.RegisterPacketHandler(serverHandler, new(pb.GateRouteClientPacketError), s.onGateRouteClientPacketError)

	serverHandler.SetUnRegisterHandler(s.routeToClient)
}

// 登录期间,playerId还没确定,这时候GatePacket.PlayerId用来存储connId
func (s *GateServer) routeToClientWithConnId(connection Connection, packet Packet) {
	gatePacket, ok := packet.(*network.GatePacket)
	if !ok {
		return
	}
	clientConn := s.getClientConnectionByConnId(uint32(gatePacket.PlayerId()))
	if clientConn == nil {
		return
	}
	// 后端→客户端:把入参 packet 的 rpcCallId 带到新建的 ProtoPacket 上
	// WithRpc 在 rpcCallId==0 时不写标记位,与改动前字节完全一致
	clientConn.SendPacket(NewProtoPacketEx(packet.Command(), packet.Message(), packet.GetStreamData()).SetErrorCode(packet.ErrorCode()).WithRpc(packet.RpcCallId()))
}

func (s *GateServer) onLoginRes(connection Connection, packet Packet) {
	res := packet.Message().(*pb.LoginRes)
	gatePacket, ok := packet.(*network.GatePacket)
	if !ok {
		return
	}
	clientConnId := uint32(gatePacket.PlayerId())
	clientConn := s.getClientConnectionByConnId(clientConnId)
	if clientConn == nil {
		slog.Debug("onLoginRes clientConnNil", "connId", clientConnId, "account", res.AccountName, "error", packet.ErrorCode())
		return
	}
	if packet.ErrorCode() == 0 {
		// 客户端登录成功,为该客户端连接设置绑定信息
		clientData := &network.ClientData{
			ConnId:    clientConn.GetConnectionId(),
			AccountId: res.AccountId,
		}
		clientData.SetGameServerId(res.GetGameServer().GetServerId())
		clientData.SetConnection(clientConn)
		clientConn.SetTag(clientData)
	}
	// 透传 rpcCallId 给客户端
	clientPacket := NewProtoPacket(packet.Command(), packet.Message()).SetErrorCode(packet.ErrorCode()).WithRpc(packet.RpcCallId())
	clientConn.SendPacket(clientPacket)
	slog.Debug("onLoginRes", "connId", clientConn.GetConnectionId(), "account", res.AccountName, "accountId", res.AccountId, "error", packet.ErrorCode(), "gameServerId", res.GetGameServer().GetServerId())
}

func (s *GateServer) onPlayerEntryGameRes(connection Connection, packet Packet) {
	res := packet.Message().(*pb.PlayerEntryGameRes)
	gatePacket, _ := packet.(*network.GatePacket)
	clientConnId := uint32(gatePacket.PlayerId())
	clientConn := s.getClientConnectionByConnId(clientConnId)
	if clientConn == nil {
		slog.Debug("onPlayerEntryGameRes clientConnNil", "connId", clientConnId, "accountId", res.AccountId, "error", packet.ErrorCode())
		return
	}
	if packet.ErrorCode() == 0 {
		if clientData, ok := clientConn.GetTag().(*network.ClientData); ok {
			// 登录游戏服成功后,绑定客户端连接和playerId,后续的消息都可以用playerId来关联
			// SetPlayerId 和 map 插入在同一个锁临界区内执行
			// 确保其他协程(如 OnConnectionDisconnect)看到 playerId > 0 时,map 必然已就绪
			s.clientsMutex.Lock()
			// 纵深防御:如果同一 playerId 已绑定旧连接(竞态导致的重复登录成功),
			// 清除旧连接的 tag 并关闭它,防止旧连接继续注入消息
			if oldClientData, ok := s.clients[res.PlayerId]; ok {
				if oldConn := oldClientData.GetConnection(); oldConn != nil && oldConn != clientConn {
					oldConn.SetTag(nil)
					oldConn.Close()
					slog.Debug("onPlayerEntryGameRes kick old conn", "oldConn", oldConn.GetConnectionId(), "playerId", res.PlayerId)
				}
			}
			clientData.SetPlayerId(res.PlayerId)
			s.clients[res.PlayerId] = clientData
			s.clientsMutex.Unlock()
			slog.Debug("bindPlayerId", "connId", clientConn.GetConnectionId(), "playerId", res.PlayerId)
		}
	}
	// 透传 rpcCallId 给客户端
	clientPacket := NewProtoPacket(packet.Command(), packet.Message()).SetErrorCode(packet.ErrorCode()).WithRpc(packet.RpcCallId())
	clientConn.SendPacket(clientPacket)
	slog.Debug("onPlayerEntryGameRes", "connId", clientConn.GetConnectionId(), "playerId", res.PlayerId, "error", packet.ErrorCode())
}

// onPlayerReconnectGameRes 处理游戏服返回的重连响应
// 重连成功时,客户端是全新连接,需要为其建立ClientData绑定(包含GameServerId)
func (s *GateServer) onPlayerReconnectGameRes(connection Connection, packet Packet) {
	res := packet.Message().(*pb.PlayerReconnectGameRes)
	gatePacket, ok := packet.(*network.GatePacket)
	if !ok {
		return
	}
	clientConnId := uint32(gatePacket.PlayerId())
	clientConn := s.getClientConnectionByConnId(clientConnId)
	if clientConn == nil {
		slog.Debug("onPlayerReconnectGameRes clientConnNil", "connId", clientConnId, "playerId", res.PlayerId, "error", packet.ErrorCode())
		return
	}
	if packet.ErrorCode() == 0 {
		// 重连成功:为新连接创建ClientData并绑定,恢复s.clients映射
		// 后续的业务消息才能通过routeToGameServer路由、GameServer下发消息才能通过routeToClient找到客户端
		clientData := &network.ClientData{
			ConnId:    clientConn.GetConnectionId(),
			AccountId: res.AccountId,
		}
		clientData.SetGameServerId(res.GameServerId)
		clientData.SetPlayerId(res.PlayerId)
		clientData.SetConnection(clientConn)
		s.clientsMutex.Lock()
		// 先清理可能残留的旧连接绑定:旧连接可能因为网络"假死"还未触发OnConnectionDisconnect
		// 清除旧连接的tag后,旧连接延迟断开时OnConnectionDisconnect会直接return(GetTag()==nil)
		// 不会误删新连接的s.clients映射,也不会发送虚假的ClientDisconnect通知
		if oldClientData, ok := s.clients[res.PlayerId]; ok {
			if oldConn := oldClientData.GetConnection(); oldConn != nil && oldConn != clientConn {
				oldConn.SetTag(nil)
				oldConn.Close()
				slog.Debug("onPlayerReconnectGameRes clear old conn", "oldConn", oldConn.GetConnectionId(), "playerId", res.PlayerId)
			}
		}
		clientConn.SetTag(clientData)
		s.clients[res.PlayerId] = clientData
		s.clientsMutex.Unlock()
		slog.Debug("onPlayerReconnectGameRes bind conn", "connId", clientConn.GetConnectionId(), "playerId", res.PlayerId, "gameServerId", res.GameServerId)
	}
	// 透传 rpcCallId 给客户端,保证重连的请求-响应配对
	clientPacket := NewProtoPacket(packet.Command(), packet.Message()).SetErrorCode(packet.ErrorCode()).WithRpc(packet.RpcCallId())
	clientConn.SendPacket(clientPacket)
	slog.Debug("onPlayerReconnectGameRes", "connId", clientConn.GetConnectionId(), "playerId", res.PlayerId, "error", packet.ErrorCode())
}

func (s *GateServer) routeToClient(connection Connection, packet Packet) {
	gatePacket, _ := packet.(*network.GatePacket)
	// 持锁期间只拷贝必要数据,释放锁后再发送,避免慢客户端阻塞写锁
	// 直接从 ClientData 获取 connection 引用,省去 getClientConnectionByConnId 的二次查找
	s.clientsMutex.RLock()
	clientData, ok := s.clients[gatePacket.PlayerId()]
	var clientConn Connection
	if ok {
		clientConn = clientData.GetConnection()
	}
	s.clientsMutex.RUnlock()
	if ok && clientConn != nil {
		// 后端→客户端:把入参 packet 的 rpcCallId 和 errorCode 都带到新建的 ProtoPacket 上
		// errorCode 必须透传,否则后端用 SendPacketAdaptWithError 设置的业务错误码会被丢弃,
		// 客户端只能靠 message 体字段判断成败,无法按 header errorCode 统一派发
		// WithRpc 在 rpcCallId==0 时不写标记位,与改动前字节完全一致
		clientConn.SendPacket(NewProtoPacketEx(packet.Command(), packet.Message(), packet.GetStreamData()).SetErrorCode(packet.ErrorCode()).WithRpc(packet.RpcCallId()))
		slog.Debug("routeToClient", "clientConn", clientData.ConnId, "playerId", clientData.GetPlayerId(), "cmd", packet.Command(), "message", packet.Message(), "dataLen", len(packet.GetStreamData()))
		return
	}
	slog.Debug("routeToClientErr", "playerId", gatePacket.PlayerId(), "cmd", packet.Command(), "message", proto.MessageName(packet.Message()), "packet", packet)
}

func (s *GateServer) getClientConnectionByConnId(clientConnId uint32) Connection {
	// Tcp和WebSocket的客户端分别由各自的Listener管理,但是ConnectionId是唯一的
	// 所以这里分别从不同的Listener查找
	// 当然,实际项目一般不会同时出现Tcp和WebSocket共存的情况
	var clientConn Connection
	if s.clientListener != nil {
		clientConn = s.clientListener.GetConnection(clientConnId)
	}
	if clientConn == nil && s.wsClientListener != nil {
		clientConn = s.wsClientListener.GetConnection(clientConnId)
	}
	return clientConn
}

// GameServer 转发客户端消息时发现玩家不在本服(例如跨服迁移/掉线重连),会回 GateRouteClientPacketError。
// Gate 收到后需要:
//  1. 清理该连接绑定的 GameServerId,避免后续继续向这个已经失效的 GameServer 转发
//  2. 转成 ErrorRes 回客户端,命令号用原请求命令号(取自 GateRouteClientPacketError.Command),
//     让客户端能识别是哪个请求失败;errorCode 用 NoPlayer,客户端可按 header errorCode 统一判断
//  3. 原样透传 rpcCallId,保证 Req/Res 配对
func (s *GateServer) onGateRouteClientPacketError(connection Connection, packet Packet) {
	gatePacket, _ := packet.(*network.GatePacket)
	errorMsg := packet.Message().(*pb.GateRouteClientPacketError)
	// 清理 GameServerId 需要写锁:GameServer 报告玩家不在本服,后续消息不应继续转发到已失效的服务器
	s.clientsMutex.Lock()
	clientData, ok := s.clients[gatePacket.PlayerId()]
	var clientConn Connection
	if ok {
		// 清理该连接绑定的 GameServerId,避免后续继续向这个已经失效的 GameServer 转发
		clientData.SetGameServerId(0)
		clientConn = clientData.GetConnection()
	}
	s.clientsMutex.Unlock()
	if !ok {
		slog.Debug("onGateRouteClientPacketError playerNotFound", "playerId", gatePacket.PlayerId(), "cmd", errorMsg.GetCommand())
		return
	}
	if clientConn == nil {
		slog.Debug("onGateRouteClientPacketError clientConnNil", "playerId", gatePacket.PlayerId())
		return
	}
	// 转成 ErrorRes:命令号取原请求命令号,reason 透传后端描述(如 "PlayerNil")
	s.sendRouteErrorRes(clientConn, PacketCommand(errorMsg.GetCommand()), packet.RpcCallId(),
		pb.ErrorCode_ErrorCode_RouteClientPacketError, errorMsg.GetResultStr())
}
