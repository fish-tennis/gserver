package common

import "github.com/fish-tennis/gnet"

type ServerConnectorHandler struct {
	gnet.DefaultConnectionHandler
	serverList *ServerList
}

func NewServerConnectorHandler(protoCodec *gnet.ProtoCodec, serverList *ServerList) *ServerConnectorHandler {
	return &ServerConnectorHandler{
		DefaultConnectionHandler: *gnet.NewDefaultConnectionHandler(protoCodec),
		serverList: serverList,
	}
}

func (this *ServerConnectorHandler) OnDisconnected(connection gnet.Connection) {
	if this.serverList != nil {
		serverId := connection.GetTag().(int32)
		this.serverList.OnServerConnectorDisconnect(serverId)
	}
}