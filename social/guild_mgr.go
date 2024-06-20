package social

import (
	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/game"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"time"
)

// 公会功能演示:
// 每个game服务器管理不同的公会,核心要点是要在分布式环境里保证同一个公会不能同时存在于不同的服务器上
// 演示代码使用redis实现的分布式锁来实现
// (很多项目会把公会功能独立成一个服务器,本质没区别)
// 演示代码演示的是一种"按需加载"的方式(lazy load),客户端请求某个公会数据时,才会把公会数据从数据库加载进内存
// 实际项目需求也可能需要服务器启动的时候就把公会数据加载进服务器(不难实现)

var (
	_guildMgr *gentity.DistributedEntityMgr
)

// DistributedEntityHelper实现
type GuildHelper struct {
}

// 加载公会实体
// 加载成功后,开启独立协程
func (g *GuildHelper) LoadEntity(entityId int64) gentity.RoutineEntity {
	return _guildMgr.LoadEntity(entityId, &pb.GuildLoadData{Id: entityId})
}

// 创建公会实体
func (g *GuildHelper) CreateEntity(entityData interface{}) gentity.RoutineEntity {
	return NewGuild(entityData.(*pb.GuildLoadData))
}

// 根据公会id路由到对应的服务器id
func (g *GuildHelper) RouteServerId(entityId int64) int32 {
	return RouteGuildServerId(entityId)
}

// 消息转换成路由消息
func (g *GuildHelper) PacketToRoutePacket(from gentity.Entity, packet Packet, toEntityId int64) Packet {
	fromPlayer := from.(*game.Player)
	return PacketToGuildRoutePacket(fromPlayer.GetId(), fromPlayer.GetName(), packet, toEntityId)
}

// 路由消息转换成公会的逻辑消息
func (g *GuildHelper) RoutePacketToRoutineMessage(connection Connection, packet Packet, toEntityId int64) interface{} {
	req := packet.Message().(*pb.GuildRoutePlayerMessageReq)
	message, err := req.PacketData.UnmarshalNew()
	if err != nil {
		logger.Error("UnmarshalNew %v err: %v", req.FromGuildId, err)
		return nil
	}
	err = req.PacketData.UnmarshalTo(message)
	if err != nil {
		logger.Error("UnmarshalTo %v err: %v", req.FromGuildId, err)
		return nil
	}
	return &GuildMessage{
		fromPlayerId:   req.FromPlayerId,
		fromServerId:   req.FromServerId,
		fromPlayerName: req.FromPlayerName,
		cmd:            PacketCommand(uint16(req.PacketCommand)),
		message:        message,
		srcPacket:      packet,
		srcConnection:  connection,
	}
}

func initGuildMgr() {
	routineArgs := &gentity.RoutineEntityRoutineArgs{
		ProcessMessageFunc: func(routineEntity gentity.RoutineEntity, routineMessage any) {
			guildMessage := routineMessage.(*GuildMessage)
			routineEntity.(*Guild).processMessage(guildMessage)
			//this.SaveCache()
			// 这里演示一种直接保存数据库的用法,可以用于那些不经常修改的数据
			// 这种方式,省去了要处理crash后从缓存恢复数据的步骤
			gentity.SaveEntityChangedDataToDb(_guildMgr.GetEntityDb(), routineEntity, cache.Get(), false, "")
		},
		AfterTimerExecuteFunc: func(routineEntity gentity.RoutineEntity, t time.Time) {
			gentity.SaveEntityChangedDataToDb(_guildMgr.GetEntityDb(), routineEntity, cache.Get(), false, "")
		},
	}
	_guildMgr = gentity.NewDistributedEntityMgr("g.lock",
		db.GetGuildDb(),
		cache.Get(),
		GetServerList(),
		routineArgs,
		new(GuildHelper))
	_guildMgr.SetLoadEntityWhenGetNil(true)
}

// 获取本服上的公会
func GetGuildById(guildId int64) *Guild {
	guild := _guildMgr.GetEntity(guildId)
	if guild == nil {
		return nil
	}
	return guild.(*Guild)
}

// 服务器列表,触发公会重新分配
func onServerListUpdate(serverList map[string][]gentity.ServerInfo, oldServerList map[string][]gentity.ServerInfo) {
	_guildMgr.ReBalance()
}
