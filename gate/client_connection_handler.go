package gate

import (
	. "github.com/fish-tennis/gnet"
)

// 玩家共用的处理接口
var _clientConnectionHandler *ClientConnectionHandler

// 客户端连接的handler
type ClientConnectionHandler struct {
	DefaultConnectionHandler
}

func NewClientConnectionHandler(protoCodec *ClientCodec) *ClientConnectionHandler {
	_clientConnectionHandler = &ClientConnectionHandler{
		DefaultConnectionHandler: *NewDefaultConnectionHandler(protoCodec),
	}
	return _clientConnectionHandler
}