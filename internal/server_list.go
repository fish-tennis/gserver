package internal

import (
	"context"
	"fmt"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
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
	// bytes -> ServerInfo
	serverInfoUnmarshal func(bytes []byte) ServerInfo
	// ServerInfo -> bytes
	serverInfoMarshal func(info ServerInfo) []byte
	// 需要获取信息的服务器类型
	fetchServerTypes []string
	// 需要连接的服务器类型
	connectServerTypes []string
	// 服务器多少毫秒没上传自己的信息,就判断为不活跃了
	activeTimeout int32
	// 缓存的服务器列表信息
	serverInfos      map[int32]ServerInfo // serverId-ServerInfo
	serverInfosMutex sync.RWMutex
	// 按照服务器类型分组的服务器列表信息
	serverInfoTypeMap      map[string][]ServerInfo
	serverInfoTypeMapMutex sync.RWMutex
	// 本地服务器信息
	localServerInfo ServerInfo
	// 已连接的服务器
	connectedServers      map[int32]gnet.Connection // serverId-Connection
	connectedServersMutex sync.RWMutex
	// 服务器连接创建函数,供外部扩展
	serverConnectorFunc func(ctx context.Context, info ServerInfo) gnet.Connection
	listUpdateHooks     []func(serverList map[string][]ServerInfo, oldServerList map[string][]ServerInfo)
}

func NewServerList() *ServerList {
	_serverList = &ServerList{
		activeTimeout:     3 * 1000, // 默认3秒
		serverInfos:       make(map[int32]ServerInfo),
		connectedServers:  make(map[int32]gnet.Connection),
		serverInfoTypeMap: make(map[string][]ServerInfo),
	}
	_serverList.SetServerInfoFunc(func(bytes []byte) ServerInfo {
		serverInfo := new(pb.ServerInfo)
		err := proto.Unmarshal(bytes, serverInfo)
		if err != nil {
			return nil
		}
		return serverInfo
	}, func(info ServerInfo) []byte {
		serverInfo := info.(*pb.ServerInfo)
		bytes, err := proto.Marshal(serverInfo)
		if err != nil {
			return nil
		}
		return bytes
	})
	return _serverList
}

func (this *ServerList) SetCache(cache gentity.KvCache) {
	this.cache = cache
}

// 设置ServerInfo的序列化接口
func (this *ServerList) SetServerInfoFunc(serverInfoUnmarshal func(bytes []byte) ServerInfo,
	serverInfoMarshal func(info ServerInfo) []byte) {
	this.serverInfoUnmarshal = serverInfoUnmarshal
	this.serverInfoMarshal = serverInfoMarshal
}

// 设置服务器连接创建函数
func (this *ServerList) SetServerConnectorFunc(connectFunc func(ctx context.Context, info ServerInfo) gnet.Connection) {
	this.serverConnectorFunc = connectFunc
}

// 服务发现: 读取服务器列表信息,并连接这些服务器
func (this *ServerList) FindAndConnectServers(ctx context.Context) {
	serverInfoMapUpdated := false
	infoMap := make(map[int32]ServerInfo)
	for _, serverType := range this.fetchServerTypes {
		serverInfoDatas := make(map[string]string)
		err := this.cache.GetMap(fmt.Sprintf("servers:%v", serverType), serverInfoDatas)
		if gentity.IsRedisError(err) {
			gentity.GetLogger().Error("get %v info err:%v", serverType, err)
			continue
		}
		for idStr, serverInfoData := range serverInfoDatas {
			serverInfo := this.serverInfoUnmarshal([]byte(serverInfoData))
			if serverInfo == nil {
				gentity.GetLogger().Error("serverInfoCreator err:k:%v v:%v", idStr, serverInfoData)
				continue
			}
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
		serverInfoTypeMap := make(map[string][]ServerInfo)
		for _, info := range infoMap {
			infoSlice, ok := serverInfoTypeMap[info.GetServerType()]
			if !ok {
				infoSlice = make([]ServerInfo, 0)
				serverInfoTypeMap[info.GetServerType()] = infoSlice
			}
			infoSlice = append(infoSlice, info)
			serverInfoTypeMap[info.GetServerType()] = infoSlice
		}
		var oldList map[string][]ServerInfo
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

// 连接其他服务器(包括自己),我方作为connector
func (this *ServerList) ConnectServer(ctx context.Context, info ServerInfo) {
	if info == nil || this.serverConnectorFunc == nil {
		return
	}
	this.connectedServersMutex.RLock()
	_, ok := this.connectedServers[info.GetServerId()]
	this.connectedServersMutex.RUnlock()
	if ok {
		return
	}
	serverConn := this.serverConnectorFunc(ctx, info)
	if serverConn != nil {
		serverConn.SetTag(info.GetServerId())
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
	bytes := this.serverInfoMarshal(this.localServerInfo)
	this.cache.HSet(fmt.Sprintf("servers:%v", this.localServerInfo.GetServerType()),
		util.Itoa(this.localServerInfo.GetServerId()), bytes)
}

// 获取某个服务器的信息
func (this *ServerList) GetServerInfo(serverId int32) ServerInfo {
	this.serverInfosMutex.RLock()
	defer this.serverInfosMutex.RUnlock()
	info, _ := this.serverInfos[serverId]
	return info
}

// 自己的服务器信息
func (this *ServerList) GetLocalServerInfo() ServerInfo {
	return this.localServerInfo
}

func (this *ServerList) SetLocalServerInfo(info ServerInfo) {
	this.localServerInfo = info
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
func (this *ServerList) GetServersByType(serverType string) []ServerInfo {
	this.serverInfoTypeMapMutex.RLock()
	defer this.serverInfoTypeMapMutex.RUnlock()
	if infoList, ok := this.serverInfoTypeMap[serverType]; ok {
		copyInfoList := make([]ServerInfo, len(infoList), len(infoList))
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
func (this *ServerList) AddListUpdateHook(onListUpdateFunc ...func(serverList map[string][]ServerInfo, oldServerList map[string][]ServerInfo)) {
	this.listUpdateHooks = append(this.listUpdateHooks, onListUpdateFunc...)
}
