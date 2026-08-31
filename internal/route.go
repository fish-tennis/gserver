package internal

import (
	"errors"
	"fmt"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"log/slog"
	"time"
)

// 路由源头消息
type RouteSourcePacket struct {
	FromPlayerId   int64
	FromServerId   int32
	FromPlayerName string
	Cmd            gnet.PacketCommand
	Message        proto.Message
	SrcPacket      gnet.Packet     // 来源packet
	SrcConnection  gnet.Connection // 来源连接,返回消息"原路返回",才能实现rpc功能;内部事件时携带触发事件的连接
	IsDirectClient bool            // 是否来自直连客户端
	InternalEvent  int             // 内部事件类型(0=普通消息),非0时 Message 为 nil,OnProcessMessage 拦截后交给 handleInternalEvent
}

// 根据公会id查找对应的服务器
// 公会实体由social服务器承载,路由到social服务器
func RouteGuildServerId(guildId int64) int32 {
	servers := GetServerList().GetServersByType(ServerType_Social)
	if len(servers) == 0 {
		return 0
	}
	// 这里只演示了最简单的路由方式
	index := guildId % int64(len(servers))
	if index < 0 {
		index += int64(len(servers))
	}
	return servers[index].GetServerId()
}

// 玩家对公会的请求消息转换成路由消息
//
//	原始消息基础上再加上一些附加数据
//	client -> game.Guild -> social.Guild
func PacketToGuildRoutePacket(fromPlayerId int64, fromPlayerName string, reqPacket gnet.Packet, guildId int64) gnet.Packet {
	anyPacket, err := anypb.New(reqPacket.Message())
	if err != nil {
		slog.Error("PacketToGuildRoutePacket anypb error", "error", err)
		return nil
	}
	routePacket := &pb.GuildRoutePlayerMessageReq{
		FromPlayerId:   fromPlayerId,
		FromGuildId:    guildId,
		FromServerId:   gentity.GetApplication().GetId(),
		FromPlayerName: fromPlayerName,
		PacketCommand:  int32(reqPacket.Command()),
		PacketData:     anyPacket,
	}
	return network.NewPacket(routePacket)
}

// 玩家的请求消息转换成路由消息
//
//	原始消息基础上再加上一些附加数据
//	client -> PlayerComponent(game) -> 其他服或其他协程
func PacketToTargetRoutePacket(fromPlayerId int64, fromPlayerName string, req proto.Message, targetEntityId int64) gnet.Packet {
	anyPacket, err := anypb.New(req)
	if err != nil {
		slog.Error("PacketToTargetRoutePacketErr", "error", err, "fromPlayerId", fromPlayerId)
		return nil
	}
	routePacket := &pb.RoutePlayerMessageReq{
		FromPlayerId: fromPlayerId,
		//FromGuildId:    0,
		FromServerId:   gentity.GetApplication().GetId(),
		FromPlayerName: fromPlayerName,
		PacketCommand:  network.GetCommandByProto(req),
		PacketData:     anyPacket,
		TargetEntityId: targetEntityId,
	}
	return network.NewPacket(routePacket)
}

// RoutePacketOption 函数式选项接口,用于配置路由参数
// 重命名为RoutePacketOption以避免与proto生成的RouteOption枚举冲突
type RoutePacketOption interface {
	apply(*routeOptions)
}

// 路由消息的参数
type routeOptions struct {
	// RouteOption位掩码,可组合RouteOption_DirectSendClient和RouteOption_SaveDb等
	Options int32

	// 指定连接
	Connection gnet.Connection

	// 路由到指定的服务器
	ToServerId int32

	Error error
}

func defaultRouteOptions() *routeOptions {
	return &routeOptions{}
}

// funcRouteOption wraps a function that modifies routeOptions into an
// implementation of the RoutePacketOption interface.
type funcRouteOption struct {
	f func(*routeOptions)
}

func (fro *funcRouteOption) apply(ro *routeOptions) {
	fro.f(ro)
}

func newFuncRouteOption(f func(*routeOptions)) *funcRouteOption {
	return &funcRouteOption{
		f: f,
	}
}

func WithDirectSendClient() RoutePacketOption {
	return newFuncRouteOption(func(options *routeOptions) {
		options.Options |= int32(pb.RouteOption_RouteOption_DirectSendClient)
	})
}

func WithSaveDb() RoutePacketOption {
	return newFuncRouteOption(func(options *routeOptions) {
		options.Options |= int32(pb.RouteOption_RouteOption_SaveDb)
	})
}

func WithToServerId(toServerId int32) RoutePacketOption {
	return newFuncRouteOption(func(options *routeOptions) {
		options.ToServerId = toServerId
	})
}

func WithConnection(connection gnet.Connection) RoutePacketOption {
	return newFuncRouteOption(func(options *routeOptions) {
		options.Connection = connection
	})
}

func WithError(err error) RoutePacketOption {
	return newFuncRouteOption(func(options *routeOptions) {
		options.Error = err
	})
}

// 路由玩家消息
// ServerA -> ServerB -> Player
//
// WithDirectSendClient():
// 消息直接转发给客户端,不做逻辑处理 ServerA -> ServerB -> Client
//
// 举例:
// 有人申请加入公会,公会广播该消息给公会成员,ServerB收到消息后,直接把消息发给客户端(Player.Send),而不需要放入玩家的逻辑消息队列(Player.OnRecvPacket)
//
// WithSaveDb(): 消息先保存数据库再转发,防止丢失
// 举例:
// 公会会长同意了玩家A的入会申请,此时玩家A可能不在线,就把该消息存入玩家的数据库,待玩家下次上线时,从数据库取出该消息,并进行相应的逻辑处理
func RoutePlayerPacket(playerId int64, packet gnet.Packet, opts ...RoutePacketOption) bool {
	log := slog.Default().With("playerId", playerId, "message", proto.MessageName(packet.Message()))
	routeOpts := defaultRouteOptions()
	for _, opt := range opts {
		opt.apply(routeOpts)
	}
	// 消息只序列化1次,SaveDb和路由共用
	var anyPacket *anypb.Any
	if packet.Message() != nil {
		var err error
		anyPacket, err = anypb.New(packet.Message())
		if err != nil {
			log.Error("RoutePlayerPacketErr anypb.New", "err", err)
			return false
		}
	}
	pendingMessageId := int64(0)
	if routeOpts.Options&int32(pb.RouteOption_RouteOption_SaveDb) != 0 {
		pendingMessageId = util.GenUniqueId()
		pendingMessage := &pb.PendingMessage{
			MessageId:     pendingMessageId, // 消息号生成唯一id
			PacketCommand: int32(packet.Command()),
			PacketData:    anyPacket,
			Timestamp:     int32(time.Now().Unix()),
		}
		pendingMessageBytes, err := proto.Marshal(pendingMessage)
		if err != nil {
			log.Error("RoutePlayerPacketErr", "err", err)
			return false
		}
		err = db.GetPlayerDb().SaveComponentField(playerId, "PendingMessages",
			util.Itoa(pendingMessage.MessageId), pendingMessageBytes)
		if err != nil {
			log.Error("RoutePlayerPacketErr", "err", err)
			return false
		}
		log.Debug("save PendingMessage", "MessageId", pendingMessage.MessageId, "cmd", packet.Command())
	}
	conn := routeOpts.Connection
	if conn == nil {
		toServerId := routeOpts.ToServerId
		if toServerId == 0 {
			_, toServerId = cache.GetOnlinePlayer(playerId)
			if toServerId == 0 {
				log.Error("RoutePlayerPacketErr player offline", "cmd", packet.Command())
				return false
			}
		}
		conn = GetServerList().GetServerConnection(toServerId)
		if conn == nil {
			log.Error("RoutePlayerPacketErr server connection nil", "cmd", packet.Command(), "toServerId", toServerId)
			return false
		}
	}
	if anyPacket == nil {
		log.Debug("anyPacketNil", "cmd", packet.Command(), "err", routeOpts.Error, "Options", routeOpts.Options)
	}
	var errStr string
	if routeOpts.Error != nil {
		errStr = routeOpts.Error.Error()
	}
	routePacket := network.NewPacket(&pb.RoutePlayerMessage{
		Error:            errStr,
		ToPlayerId:       playerId,
		PacketCommand:    int32(packet.Command()),
		Options:          routeOpts.Options,
		PendingMessageId: pendingMessageId,
		PacketData:       anyPacket,
	})
	if protoPacket, ok := packet.(*gnet.ProtoPacket); ok {
		routePacket.SetRpcCallId(protoPacket.RpcCallId())
	}
	return conn.SendPacket(routePacket)
}

// RoutePlayerPackets 批量路由同一消息给多个玩家
// 优化:用1次Redis MGET替代N次GET,消息只序列化1次
// playerIds:目标玩家列表, message:要路由的消息, opts:路由选项(所有玩家共用)
func RoutePlayerPackets(playerIds []int64, packet gnet.Packet, opts ...RoutePacketOption) {
	if len(playerIds) == 0 {
		return
	}
	routeOpts := defaultRouteOptions()
	for _, opt := range opts {
		opt.apply(routeOpts)
	}
	// 消息只序列化1次
	var anyPacket *anypb.Any
	if packet.Message() != nil {
		var err error
		anyPacket, err = anypb.New(packet.Message())
		if err != nil {
			slog.Error("RoutePlayerPackets anypb.New err", "err", err)
			return
		}
	}
	var errStr string
	if routeOpts.Error != nil {
		errStr = routeOpts.Error.Error()
	}
	cmd := int32(packet.Command())
	// 1次MGET批量查询所有玩家所在的服务器
	serverMap := cache.GetOnlinePlayers(playerIds)
	// 按服务器分组发送
	for _, playerId := range playerIds {
		toServerId, ok := serverMap[playerId]
		if !ok {
			// 玩家不在线:如果启用了 SaveDb,保存 PendingMessage 待下次上线处理
			if routeOpts.Options&int32(pb.RouteOption_RouteOption_SaveDb) != 0 && anyPacket != nil {
				pendingMessageId := util.GenUniqueId()
				pendingMessage := &pb.PendingMessage{
					MessageId:     pendingMessageId,
					PacketCommand: cmd,
					PacketData:    anyPacket,
					Timestamp:     int32(time.Now().Unix()),
				}
				pendingMessageBytes, err := proto.Marshal(pendingMessage)
				if err != nil {
					slog.Error("RoutePlayerPackets marshal err", "playerId", playerId, "err", err)
					continue
				}
				err = db.GetPlayerDb().SaveComponentField(playerId, "PendingMessages",
					util.Itoa(pendingMessage.MessageId), pendingMessageBytes)
				if err != nil {
					slog.Error("RoutePlayerPackets save pending err", "playerId", playerId, "err", err)
				}
			}
			continue
		}
		conn := GetServerList().GetServerConnection(toServerId)
		if conn == nil {
			continue
		}
		routePacket := network.NewPacket(&pb.RoutePlayerMessage{
			Error:            errStr,
			ToPlayerId:       playerId,
			PacketCommand:    cmd,
			Options:          routeOpts.Options,
			PacketData:       anyPacket,
		})
		conn.SendPacket(routePacket)
	}
}

// RoutePlayerPacketRpc 通过Rpc发送路由消息并等待结果
// reply为期望的Res proto类型(如&pb.GmSetLevelRes{})
// gameserver的onRoutePlayerMessage会投递到玩家协程执行,执行完后通过来源connection做rpc reply
// 全程不阻塞网络协程:网络协程仅投递消息,玩家协程内执行handler并做rpc reply
func RoutePlayerPacketRpc(playerId int64, packet gnet.Packet, reply proto.Message) error {
	return RoutePlayerPacketRpcTimeout(playerId, packet, reply, 0)
}

// RoutePlayerPacketRpcTimeout 与RoutePlayerPacketRpc功能相同,额外支持自定义等待回复的超时时间
// timeout<=0表示使用DefaultRpcTimeout
func RoutePlayerPacketRpcTimeout(playerId int64, packet gnet.Packet, reply proto.Message, timeout time.Duration) error {
	_, toServerId := cache.GetOnlinePlayer(playerId)
	if toServerId == 0 {
		return errors.New("玩家不在线")
	}
	// 把原始消息序列化成any,放进RoutePlayerMessage中路由
	var anyPacket *anypb.Any
	if packet.Message() != nil {
		var err error
		anyPacket, err = anypb.New(packet.Message())
		if err != nil {
			return fmt.Errorf("anypb.New err: %w", err)
		}
	}
	routeMsg := &pb.RoutePlayerMessage{
		ToPlayerId:    playerId,
		PacketCommand: int32(packet.Command()),
		PacketData:    anyPacket,
	}
	// 同步Rpc:内部会在收到reply后唤醒当前协程,网络协程不被阻塞
	routeReply := new(pb.RoutePlayerMessage)
	err := GetServerList().RpcTimeout(toServerId, network.NewPacket(routeMsg), routeReply, timeout)
	if err != nil {
		return err
	}
	if routeReply.Error != "" {
		return errors.New(routeReply.Error)
	}
	if routeReply.PacketData == nil {
		return nil
	}
	// 把reply中的any反序列化到调用方提供的proto对象
	return routeReply.PacketData.UnmarshalTo(reply)
}
