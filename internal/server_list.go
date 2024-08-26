package internal

import (
	"context"
	"fmt"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"slices"
	"sort"
	"sync"
)

var (
	// singleton
	_serverList *ServerList
)

// singleton
func GetServerList() *ServerList {
	return _serverList
}

// 服务器信息接口
type ServerInfo interface {
	GetServerId() int32
	GetServerType() string
	GetLastActiveTime() int64
}

// 服务器列表管理
// 每个服务器定时上传自己的信息到redis,其他服务器定时从redis获取整个服务器集群的信息
// 属于服务注册和发现的功能,zookeeper的临时节点更适合来实现这类需求
// 这里用redis来实现,pb.ServerInfo.LastActiveTime记录服务器最后上传信息的时间,达到类似"心跳检测"的效果
type ServerList struct {
	// 缓存接口
	cache gentity.KvCache
	// 需要获取信息的服务器类型
	fetchServerTypes []string
	// 需要连接的服务器类型
	connectServerTypes []string
	// 服务器多少毫秒没上传自己的信息,就判断为不活跃了
	activeTimeout int32
	// 缓存的服务器列表信息
	serverInfos      map[int32]*pb.ServerInfo // serverId-ServerInfo
	serverInfosMutex sync.RWMutex
	// 按照服务器类型分组的服务器列表信息
	serverInfoTypeMap      map[string][]*pb.ServerInfo
	serverInfoTypeMapMutex sync.RWMutex
	// 本地服务器信息
	localServerInfo *pb.ServerInfo
	// 服务器的监听配置
	serverListenerConfig gnet.ListenerConfig
	// 服务器之间的连接配置
	serverConnectorConfig gnet.ConnectionConfig
	// 服务器listener
	serverListener gnet.Listener
	// 已连接的服务器
	connectedServers      map[int32]gnet.Connection // serverId-Connection
	connectedServersMutex sync.RWMutex
	// 服务器连接创建函数,供外部扩展
	listUpdateHooks []func(serverList map[string][]*pb.ServerInfo, oldServerList map[string][]*pb.ServerInfo)
}

func NewServerList(serverInfo *pb.ServerInfo) *ServerList {
	_serverList = &ServerList{
		activeTimeout:         3 * 1000, // 默认3秒
		serverInfos:           make(map[int32]*pb.ServerInfo),
		connectedServers:      make(map[int32]gnet.Connection),
		serverInfoTypeMap:     make(map[string][]*pb.ServerInfo),
		localServerInfo:       serverInfo,
		serverConnectorConfig: network.ServerConnectionConfig,
	}
	// 初始化服务器之间的网络配置
	_serverList.initDefaultServerConnectorConfig()
	_serverList.initDefaultServerListenerConfig()
	return _serverList
}

func (this *ServerList) SetCache(cache gentity.KvCache) {
	this.cache = cache
}

func (this *ServerList) initDefaultServerConnectorConfig() {
	var codec gnet.Codec
	if this.localServerInfo.ServerType == ServerType_Gate {
		// gate -> otherServer
		codec = network.NewGateCodec(nil)
	} else {
		// otherServer -> otherServer
		codec = gnet.NewProtoCodec(nil)
	}
	handler := gnet.NewDefaultConnectionHandler(codec)
	handler.SetOnConnectedFunc(func(connection gnet.Connection, success bool) {
		slog.Info("OnConnectedServer", "RemoteAddr", connection.RemoteAddr().String(), "success", success)
		if !success {
			return
		}
		// 连接成功后,告诉对方自己的服务器id和type
		connection.SendPacket(this.NewAdaptPacket(gnet.PacketCommand(pb.CmdInner_Cmd_ServerHello), &pb.ServerHello{
			ServerId:   this.GetLocalServerInfo().GetServerId(),
			ServerType: this.GetLocalServerInfo().GetServerType(),
		}))
	})
	handler.SetOnDisconnectedFunc(func(connection gnet.Connection) {
		if connection.GetTag() == nil {
			return
		}
		serverId := connection.GetTag().(int32)
		this.OnServerConnectorDisconnect(serverId)
	})
	if this.localServerInfo.ServerType == ServerType_Gate {
		// gate -> otherServer
		handler.RegisterHeartBeat(func() gnet.Packet {
			return network.NewGatePacket(0, gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), &pb.HeartBeatReq{
				Timestamp: util.GetCurrentMS(),
			})
		})
	} else {
		handler.RegisterHeartBeat(func() gnet.Packet {
			return this.NewAdaptPacket(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), &pb.HeartBeatReq{
				Timestamp: util.GetCurrentMS(),
			})
		})
	}
	handler.Register(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatRes), func(connection gnet.Connection, packet gnet.Packet) {
		// TODO: set ping (ServerInfo)
	}, new(pb.HeartBeatRes))
	this.serverConnectorConfig = network.ServerConnectionConfig
	this.serverConnectorConfig.Codec = codec
	this.serverConnectorConfig.Handler = handler
}

func (this *ServerList) initDefaultServerListenerConfig() {
	// listener的codec和handler
	listenerCodec := gnet.NewProtoCodec(nil)
	acceptServerHandler := gnet.NewDefaultConnectionHandler(listenerCodec)
	this.serverListenerConfig.AcceptConfig = network.ServerConnectionConfig
	this.serverListenerConfig.AcceptConfig.Codec = listenerCodec
	this.serverListenerConfig.AcceptConfig.Handler = acceptServerHandler
	acceptServerHandler.Register(gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), func(connection gnet.Connection, packet gnet.Packet) {
		req := packet.Message().(*pb.HeartBeatReq)
		network.SendPacketAdapt(connection, packet, gnet.PacketCommand(pb.CmdInner_Cmd_HeartBeatRes), &pb.HeartBeatRes{
			RequestTimestamp:  req.GetTimestamp(),
			ResponseTimestamp: util.GetCurrentMS(),
		})
	}, new(pb.HeartBeatReq))
	acceptServerHandler.Register(gnet.PacketCommand(pb.CmdInner_Cmd_ServerHello), func(connection gnet.Connection, packet gnet.Packet) {
		serverHello := packet.Message().(*pb.ServerHello)
		this.OnServerConnected(serverHello.ServerId, connection)
		slog.Info("AcceptServer", "serverHello", serverHello, "connId", connection.GetConnectionId())
	}, new(pb.ServerHello))
}

func (this *ServerList) GetServerConnectionHandler() *gnet.DefaultConnectionHandler {
	return this.serverConnectorConfig.Handler.(*gnet.DefaultConnectionHandler)
}

func (this *ServerList) GetServerListenerHandler() *gnet.DefaultConnectionHandler {
	return this.serverListenerConfig.AcceptConfig.Handler.(*gnet.DefaultConnectionHandler)
}

func (this *ServerList) NewAdaptPacket(cmd gnet.PacketCommand, message proto.Message) gnet.Packet {
	if this.localServerInfo.ServerType == ServerType_Gate {
		return network.NewGatePacket(0, cmd, message)
	} else {
		return gnet.NewProtoPacket(cmd, message)
	}
}

// 服务发现: 读取服务器列表信息,并连接这些服务器
func (this *ServerList) FindAndConnectServers(ctx context.Context) {
	serverInfoMapUpdated := false
	infoMap := make(map[int32]*pb.ServerInfo)
	for _, serverType := range this.fetchServerTypes {
		serverInfos := make(map[int32]*pb.ServerInfo)
		err := this.cache.GetMap(fmt.Sprintf("servers:%v", serverType), serverInfos)
		if gentity.IsRedisError(err) {
			gentity.GetLogger().Error("get %v info err:%v", serverType, err)
			continue
		}
		for _, serverInfo := range serverInfos {
			// 目标服务器已经处于"不活跃"状态了
			if util.GetCurrentMS()-serverInfo.GetLastActiveTime() > int64(this.activeTimeout) {
				continue
			}
			// 这里不用加锁,因为其他协程不会修改serverInfos
			if _, ok := this.serverInfos[serverInfo.GetServerId()]; !ok {
				serverInfoMapUpdated = true
			}
			infoMap[serverInfo.GetServerId()] = serverInfo
		}
	}
	if len(this.serverInfos) != len(infoMap) {
		serverInfoMapUpdated = true
	}
	// 服务器列表有更新,才更新服务器列表和类型分组信息
	if serverInfoMapUpdated {
		this.serverInfosMutex.Lock()
		this.serverInfos = infoMap
		this.serverInfosMutex.Unlock()
		serverInfoTypeMap := make(map[string][]*pb.ServerInfo)
		for _, info := range infoMap {
			infoSlice, ok := serverInfoTypeMap[info.GetServerType()]
			if !ok {
				infoSlice = make([]*pb.ServerInfo, 0)
				serverInfoTypeMap[info.GetServerType()] = infoSlice
			}
			infoSlice = append(infoSlice, info)
			serverInfoTypeMap[info.GetServerType()] = infoSlice
		}
		var oldList map[string][]*pb.ServerInfo
		this.serverInfoTypeMapMutex.Lock()
		oldList = this.serverInfoTypeMap
		this.serverInfoTypeMap = serverInfoTypeMap
		this.serverInfoTypeMapMutex.Unlock()
		for _, hookFunc := range this.listUpdateHooks {
			hookFunc(serverInfoTypeMap, oldList)
		}
	}

	for _, info := range infoMap {
		if slices.Contains(this.connectServerTypes, info.GetServerType()) {
			//// 目标服务器已经处于"不活跃"状态了
			//if util.GetCurrentMS() - info.LastActiveTime > int64(this.activeTimeout) {
			//	continue
			//}
			// 只连接Id>=自己的服务器,让每2个服务器之间只有一条链接
			// NOTE: 自己和自己也会产生一条链接
			if info.GetServerId() < this.localServerInfo.GetServerId() {
				continue
			}
			this.ConnectServer(ctx, info)
		}
	}
}

// 开始监听服务器
func (this *ServerList) StartListen(ctx context.Context, serverListenAddr string) gnet.Listener {
	this.serverListener = gnet.GetNetMgr().NewListener(ctx, serverListenAddr, &this.serverListenerConfig)
	return this.serverListener
}

// 连接其他服务器(包括自己),我方作为connector
func (this *ServerList) ConnectServer(ctx context.Context, info *pb.ServerInfo) {
	if info == nil {
		return
	}
	this.connectedServersMutex.RLock()
	_, ok := this.connectedServers[info.GetServerId()]
	this.connectedServersMutex.RUnlock()
	if ok {
		return
	}
	targetAddr := info.GetServerListenAddr()
	if this.localServerInfo.GetServerType() == ServerType_Gate {
		// gate -> otherServer
		targetAddr = info.GetGateListenAddr()
	}
	serverConn := gnet.GetNetMgr().NewConnector(ctx, targetAddr, &this.serverConnectorConfig, info.GetServerId())
	if serverConn != nil {
		this.connectedServersMutex.Lock()
		this.connectedServers[info.GetServerId()] = serverConn
		this.connectedServersMutex.Unlock()
		gentity.GetLogger().Info("ConnectServer %v, %v", info.GetServerId(), info.GetServerType())
	} else {
		gentity.GetLogger().Info("ConnectServerError %v, %v", info.GetServerId(), info.GetServerType())
	}
}

// 服务注册:上传本地服务器的信息
func (this *ServerList) RegisterLocalServerInfo() {
	bytes, _ := proto.Marshal(this.localServerInfo)
	this.cache.HSet(fmt.Sprintf("servers:%v", this.localServerInfo.GetServerType()),
		util.Itoa(this.localServerInfo.GetServerId()), bytes)
}

// 获取某个服务器的信息
func (this *ServerList) GetServerInfo(serverId int32) *pb.ServerInfo {
	this.serverInfosMutex.RLock()
	defer this.serverInfosMutex.RUnlock()
	info, _ := this.serverInfos[serverId]
	return info
}

// 自己的服务器信息
func (this *ServerList) GetLocalServerInfo() *pb.ServerInfo {
	return this.localServerInfo
}

// 服务器连接断开了
func (this *ServerList) OnServerConnectorDisconnect(serverId int32) {
	this.connectedServersMutex.Lock()
	delete(this.connectedServers, serverId)
	this.connectedServersMutex.Unlock()
	gentity.GetLogger().Debug("DisconnectServer %v", serverId)
}

// 其他服务器连接上,我方作为listener
func (this *ServerList) OnServerConnected(serverId int32, connection gnet.Connection) {
	connection.SetTag(serverId)
	// 当自己连接自己时,会产生2条相同serverId的连接:connector和accept connection
	// 这里只会保存其中一个 TODO: 怎么处理?
	//// 只保留connector的连接信息
	//if serverId == this.localServerInfo.GetServerId() && !connection.IsConnector() {
	//	GetLogger().Debug("Ignore self accept %v", serverId)
	//	return
	//}
	this.connectedServersMutex.Lock()
	this.connectedServers[serverId] = connection
	this.connectedServersMutex.Unlock()
	gentity.GetLogger().Debug("OnServerConnected %v", serverId)
}

// 设置要获取的服务器类型
func (this *ServerList) SetFetchServerTypes(serverTypes ...string) {
	this.fetchServerTypes = append(this.fetchServerTypes, serverTypes...)
	gentity.GetLogger().Debug("fetch:%v", serverTypes)
}

// 设置要获取并连接的服务器类型
func (this *ServerList) SetFetchAndConnectServerTypes(serverTypes ...string) {
	this.fetchServerTypes = append(this.fetchServerTypes, serverTypes...)
	this.connectServerTypes = append(this.connectServerTypes, serverTypes...)
	gentity.GetLogger().Info("fetch connect:%v", serverTypes)
}

// 获取某类服务器的信息列表
func (this *ServerList) GetServersByType(serverType string) []*pb.ServerInfo {
	this.serverInfoTypeMapMutex.RLock()
	defer this.serverInfoTypeMapMutex.RUnlock()
	if infoList, ok := this.serverInfoTypeMap[serverType]; ok {
		copyInfoList := make([]*pb.ServerInfo, len(infoList), len(infoList))
		for idx, info := range infoList {
			copyInfoList[idx] = info
		}
		sort.Slice(copyInfoList, func(i, j int) bool {
			return copyInfoList[i].GetServerId() < copyInfoList[j].GetServerId()
		})
		return copyInfoList
	}
	return nil
}

// 获取服务器的连接
func (this *ServerList) GetServerConnection(serverId int32) gnet.Connection {
	this.connectedServersMutex.RLock()
	connection, _ := this.connectedServers[serverId]
	this.connectedServersMutex.RUnlock()
	return connection
}

// 发消息给另一个服务器
func (this *ServerList) Send(serverId int32, cmd gnet.PacketCommand, message proto.Message, opts ...gnet.SendOption) bool {
	connection := this.GetServerConnection(serverId)
	if connection != nil && connection.IsConnected() {
		return connection.Send(cmd, message, opts...)
	}
	return false
}

func (this *ServerList) SendPacket(serverId int32, packet gnet.Packet, opts ...gnet.SendOption) bool {
	connection := this.GetServerConnection(serverId)
	if connection != nil && connection.IsConnected() {
		return connection.SendPacket(packet, opts...)
	}
	return false
}

func (this *ServerList) Rpc(serverId int32, request gnet.Packet, reply proto.Message, opts ...gnet.SendOption) error {
	connection := this.GetServerConnection(serverId)
	if connection != nil && connection.IsConnected() {
		return connection.Rpc(request, reply, opts...)
	}
	return gentity.ErrNotConnected
}

// 添加服务器列表更新回调
func (this *ServerList) AddListUpdateHook(onListUpdateFunc ...func(serverList map[string][]*pb.ServerInfo, oldServerList map[string][]*pb.ServerInfo)) {
	this.listUpdateHooks = append(this.listUpdateHooks, onListUpdateFunc...)
}
