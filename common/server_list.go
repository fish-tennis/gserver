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
	// 缓存的服务器列表信息
	serverInfosMutex sync.RWMutex
	serverInfos map[int32]*pb.ServerInfo // serverId-*pb.ServerInfo
	// 自己的服务器信息
	localServerInfo *pb.ServerInfo
	// 已连接的服务器
	connectedServersMutex sync.RWMutex
	connectedServers map[int32]uint32 // serverId-connectionId
	// 服务器连接配置
	serverConnectionConfig gnet.ConnectionConfig
	// 服务器handler
	serverConnectorHandler gnet.ConnectionHandler
}

func NewServerList() *ServerList {
	serverList := &ServerList{
		serverInfos: make(map[int32]*pb.ServerInfo),
		connectedServers: make(map[int32]uint32),
	}
	// 服务器之间使用默认的编码
	serverConnectorHandler := NewServerConnectorHandler(gnet.NewProtoCodec(nil), serverList)
	serverList.serverConnectorHandler = serverConnectorHandler
	return serverList
}

// 读取服务器列表信息
func (this *ServerList) FetchServerInfos() {
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
			infoMap[serverInfo.GetServerId()] = serverInfo
		}
	}
	this.serverInfosMutex.Lock()
	this.serverInfos = infoMap
	this.serverInfosMutex.Unlock()

	for _,info := range infoMap {
		if util.HasString(this.connectServerTypes, info.GetServerType()) {
			if this.localServerInfo.GetServerId() == info.GetServerId() {
				continue
			}
			this.ConnectServer(info)
		}
	}
}

func (this *ServerList) ConnectServer(info *pb.ServerInfo) {
	if info == nil {
		return
	}
	this.connectedServersMutex.RLock()
	_,ok := this.connectedServers[info.GetServerId()]
	this.connectedServersMutex.RUnlock()
	if ok {
		return
	}
	serverConn := gnet.GetNetMgr().NewConnector(info.ServerListenAddr, this.serverConnectionConfig,
		gnet.NewDefaultCodec(), this.serverConnectorHandler)
	if serverConn != nil {
		serverConn.SetTag(info.GetServerId())
		this.connectedServersMutex.Lock()
		this.connectedServers[info.GetServerId()] = serverConn.GetConnectionId()
		this.connectedServersMutex.Unlock()
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

func (this *ServerList) OnServerConnectorDisconnect(serverId int32) {
	this.connectedServersMutex.Lock()
	delete(this.connectedServers, serverId)
	this.connectedServersMutex.Unlock()
}