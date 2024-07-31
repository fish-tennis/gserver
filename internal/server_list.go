package internal

import (
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
)

var (
	// singleton
	_serverList *ServerList
)

// 服务器列表管理
// 每个服务器定时上传自己的信息到redis,其他服务器定时从redis获取整个服务器集群的信息
// 属于服务注册和发现的功能,zookeeper的临时节点更适合来实现这类需求
// 这里用redis来实现,pb.ServerInfo.LastActiveTime记录服务器最后上传信息的时间,达到类似"心跳检测"的效果
type ServerList struct {
	BaseServerList
}

func NewServerList() *ServerList {
	_serverList = &ServerList{
		BaseServerList: *NewBaseServerList(),
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

// singleton
func GetServerList() *ServerList {
	return _serverList
}
