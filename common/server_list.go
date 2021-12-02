package common

import (
	"context"
	"fmt"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"google.golang.org/protobuf/proto"
	"sync"
)

// 服务器列表管理
// 每个服务器定时上传自己的信息到redis,其他服务器定时从redis获取整个服务器集群的信息
type ServerList struct {
	// 需要获取信息的服务器类型
	fetchServerTypes []string
	// 需要连接的服务器类型
	connectServerTypes []string
	// 服务器多少毫秒没上传自己的信息,就判断为不活跃了
	activeTimeout int32
	// 缓存的服务器列表信息
	serverInfos map[int32]*pb.ServerInfo // serverId-*pb.ServerInfo
	serverInfosMutex sync.RWMutex
	// 按照服务器类型分组的服务器列表信息
	serverInfoTypeMap map[string][]*pb.ServerInfo
	serverInfoTypeMapMutex sync.RWMutex
	// 自己的服务器信息
	localServerInfo *pb.ServerInfo
	// 已连接的服务器
	connectedServerConnectors      map[int32]gnet.Connection // serverId-Connection
	connectedServerConnectorsMutex sync.RWMutex
	// 服务器连接创建函数,供外部扩展
	serverConnectorFunc func(info *pb.ServerInfo) gnet.Connection
}

func NewServerList() *ServerList {
	serverList := &ServerList{
		activeTimeout:             3 * 1000, // 默认3秒
		serverInfos:               make(map[int32]*pb.ServerInfo),
		connectedServerConnectors: make(map[int32]gnet.Connection),
		serverInfoTypeMap:         make(map[string][]*pb.ServerInfo),
	}
	return serverList
}

// 读取服务器列表信息,并连接这些服务器
func (this *ServerList) FetchAndConnectServers() {
	serverInfoMapUpdated := false
	infoMap := make(map[int32]*pb.ServerInfo)
	for _,serverType := range this.fetchServerTypes {
		serverInfoDatas, err := cache.GetRedis().HVals(context.TODO(), fmt.Sprintf("servers:%v",serverType)).Result()
		if cache.IsRedisError(err) {
			gnet.LogError("%v", err)
			continue
		}
		for _,serverInfoData := range serverInfoDatas {
			serverInfo := new(pb.ServerInfo)
			decodeErr := proto.Unmarshal([]byte(serverInfoData), serverInfo)
			if decodeErr != nil {
				gnet.LogError("%v", decodeErr)
				continue
			}
			// 目标服务器已经处于"不活跃"状态了
			if util.GetCurrentMS() - serverInfo.LastActiveTime > int64(this.activeTimeout) {
				continue
			}
			// 这里不用加锁,因为其他协程不会修改serverInfos
			if _,ok := this.serverInfos[serverInfo.GetServerId()]; !ok {
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
		for _,info := range infoMap {
			infoSlice,ok := serverInfoTypeMap[info.GetServerType()]
			if !ok {
				infoSlice = make([]*pb.ServerInfo, 0)
				serverInfoTypeMap[info.GetServerType()] = infoSlice
			}
			infoSlice = append(infoSlice, info)
			serverInfoTypeMap[info.GetServerType()] = infoSlice
		}
		this.serverInfoTypeMapMutex.Lock()
		this.serverInfoTypeMap = serverInfoTypeMap
		this.serverInfoTypeMapMutex.Unlock()
	}

	for _,info := range infoMap {
		if util.HasString(this.connectServerTypes, info.GetServerType()) {
			if this.localServerInfo.GetServerId() == info.GetServerId() {
				continue
			}
			//// 目标服务器已经处于"不活跃"状态了
			//if util.GetCurrentMS() - info.LastActiveTime > int64(this.activeTimeout) {
			//	continue
			//}
			this.ConnectServer(info)
		}
	}
}

// 连接其他服务器
func (this *ServerList) ConnectServer(info *pb.ServerInfo) {
	if info == nil || this.serverConnectorFunc == nil {
		return
	}
	this.connectedServerConnectorsMutex.RLock()
	_,ok := this.connectedServerConnectors[info.GetServerId()]
	this.connectedServerConnectorsMutex.RUnlock()
	if ok {
		return
	}
	serverConn := this.serverConnectorFunc(info)
	if serverConn != nil {
		serverConn.SetTag(info.GetServerId())
		this.connectedServerConnectorsMutex.Lock()
		this.connectedServerConnectors[info.GetServerId()] = serverConn
		this.connectedServerConnectorsMutex.Unlock()
		gnet.LogDebug("ConnectServer %v, %v", info.GetServerId(), info.ServerListenAddr)
	} else {
		gnet.LogDebug("ConnectServerError %v, %v", info.GetServerId(), info.ServerListenAddr)
	}
}

// 上传某个服务器的信息
func (this *ServerList) UploadServerInfo(info *pb.ServerInfo) {
	data,_ := proto.Marshal(info)
	cache.GetRedis().HSet(context.TODO(), fmt.Sprintf("servers:%v",info.GetServerType()),
		info.GetServerId(), data)
}

// 获取某个服务器的信息
func (this *ServerList) GetServerInfo(serverId int32) *pb.ServerInfo {
	this.serverInfosMutex.RLock()
	defer this.serverInfosMutex.RUnlock()
	info,_ := this.serverInfos[serverId]
	return info
}

// 服务器连接断开了
func (this *ServerList) OnServerConnectorDisconnect(serverId int32) {
	this.connectedServerConnectorsMutex.Lock()
	delete(this.connectedServerConnectors, serverId)
	this.connectedServerConnectorsMutex.Unlock()
	gnet.LogDebug("DisconnectServer %v", serverId)
}

// 设置要获取的服务器类型
func (this *ServerList) SetFetchServerTypes( serverTypes ...string) {
	this.fetchServerTypes = append(this.fetchServerTypes, serverTypes...)
	gnet.LogDebug("fetch:%v", serverTypes)
}

// 设置要获取并连接的服务器类型
func (this *ServerList) SetFetchAndConnectServerTypes( serverTypes ...string) {
	this.fetchServerTypes = append(this.fetchServerTypes, serverTypes...)
	this.connectServerTypes = append(this.connectServerTypes, serverTypes...)
	gnet.LogDebug("fetch connect:%v", serverTypes)
}

// 获取某类服务器的信息列表
func (this *ServerList) GetServersByType(serverType string) []*pb.ServerInfo {
	this.serverInfoTypeMapMutex.RLock()
	infoList,_ := this.serverInfoTypeMap[serverType]
	this.serverInfoTypeMapMutex.RUnlock()
	return infoList
}

// 获取服务器的连接
func (this *ServerList) GetServerConnector(serverId int32 ) gnet.Connection {
	this.connectedServerConnectorsMutex.RLock()
	connection,_ := this.connectedServerConnectors[serverId]
	this.connectedServerConnectorsMutex.RUnlock()
	return connection
}