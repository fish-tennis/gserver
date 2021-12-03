package common

import (
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/entity"
)

// 简化类名
type Packet = gnet.Packet
type ProtoPacket = gnet.ProtoPacket
type Cmd = gnet.PacketCommand
type Connection = gnet.Connection
type Component = entity.Component
type Entity = entity.Entity
type EventReceiver = entity.EventReceiver

var (
	LogDebug = gnet.LogDebug
	LogError = gnet.LogError
)