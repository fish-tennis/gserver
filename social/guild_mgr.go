package social

import (
	"context"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/db/mongodb"
	"github.com/fish-tennis/gserver/gameplayer"
	"github.com/fish-tennis/gserver/gen"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/protobuf/types/known/anypb"
	"math"
	"sync"
)

// 公会功能演示:
// 每个game服务器管理不同的公会,核心要点是要在分布式环境里保证同一个公会不能同时存在于不同的服务器上
// 演示代码使用redis实现的分布式锁来实现
// (很多项目会把公会功能独立成一个服务器,本质没区别)
// 演示代码演示的是一种"按需加载"的方式,客户端请求某个公会数据时,才会把公会数据从数据库加载进内存
// 实际项目需求也可能需要服务器启动的时候就把公会数据加载进服务器(不难实现)

var (
	_guildMap     = make(map[int64]*Guild)
	_guildMapLock sync.RWMutex
)

// 获取本服上的公会
func GetGuildById(guildId int64) *Guild {
	_guildMapLock.RLock()
	defer _guildMapLock.RUnlock()
	return _guildMap[guildId]
}

// 从数据库加载公会数据
func LoadGuild(guildId int64) *Guild {
	guildData := &pb.GuildLoadData{
		Id: guildId,
	}
	exist, err := GetGuildDb().FindEntityById(guildId, guildData)
	if err != nil {
		logger.Error("LoadGuild err:%v", err)
		return nil
	}
	if !exist {
		return nil
	}
	guild := NewGuild(guildData)
	_guildMapLock.Lock()
	defer _guildMapLock.Unlock()
	if existGuild, ok := _guildMap[guildId]; ok {
		return existGuild
	}
	if guild.RunProcessRoutine() {
		_guildMap[guildId] = guild
	}
	return guild
}

// 公会db接口
func GetGuildDb() db.EntityDb {
	return db.GetDbMgr().GetEntityDb("guild")
}

// 服务器动态扩缩容了,公会重新分配
func onServerListUpdate(serverList map[string][]*pb.ServerInfo, oldServerList map[string][]*pb.ServerInfo) {
	_guildMapLock.RLock()
	defer _guildMapLock.RUnlock()
	for _, guild := range _guildMap {
		if GuildRoute(guild.GetId()) != GetServer().GetServerId() {
			// 通知已不属于本服务器管理的公会关闭协程
			guild.Stop()
			logger.Info("stop guild %v", guild.GetId())
		}
	}
}

// redis实现的分布式锁,保证同一个公会的逻辑处理协程只会在一个服务器上
func guildServerLock(guildId int64) bool {
	// redis实现的分布式锁,保证同一个公会的逻辑处理协程只会在一个服务器上
	// 锁的是公会id和服务器id的对应关系
	lockOK, err := cache.GetRedis().HSetNX(context.Background(), "g.lock", util.Itoa(guildId), GetServer().GetServerId()).Result()
	if cache.IsRedisError(err) {
		logger.Error("guild %v lock err:%v", guildId, err.Error())
		return false
	}
	if !lockOK {
		logger.Error("guild %v lock failed", guildId)
		return false
	}
	logger.Debug("guildServerLock %v", guildId)
	return true
}

// 分布式锁UnLock
func guildServerUnlock(guildId int64) {
	cache.GetRedis().HDel(context.Background(), "g.lock", util.Itoa(guildId))
	logger.Debug("guildServerUnlock %v", guildId)
}

// 删除跟本服关联的分布式锁
func deleteGuildServerLock() {
	kv, err := cache.GetRedis().HGetAll(context.Background(), "g.lock").Result()
	if cache.IsRedisError(err) {
		logger.Error("ResetGuildLock err:%v", err.Error())
		return
	}
	for guildIdStr, serverIdStr := range kv {
		if util.Atoi(serverIdStr) == int(GetServer().GetServerId()) {
			cache.GetRedis().HDel(context.Background(), "g.lock", guildIdStr)
			logger.Debug("deleteGuildServerLock %v", guildIdStr)
		}
	}
}

// 根据公会id路由到对应的服务器id
func GuildRoute(guildId int64) int32 {
	servers := GetServerList().GetServersByType("game")
	// 这里只演示了最简单的路由方式
	// 实际项目可能采用一致性哈希等其他方式
	index := guildId % int64(len(servers))
	return servers[index].ServerId
}

// 根据公会id路由玩家的请求消息
func GuildRouteReqPacket(player *gameplayer.Player, guildId int64, packet *ProtoPacket) bool {
	routeServerId := GuildRoute(guildId)
	logger.Debug("GuildRouteReqPacket %v -> %v", guildId, routeServerId)
	if routeServerId <= 0 {
		return false
	}
	// 属于本服务器管理的公会
	if GetServer().GetServerId() == routeServerId {
		// 先从内存中查找
		guild := GetGuildById(guildId)
		if guild == nil {
			// 再到数据库加载
			guild = LoadGuild(guildId)
			if guild == nil {
				logger.Error("not find guild:%v", guildId)
				return false
			}
		}
		guild.PushMessage(&GuildMessage{
			fromPlayerId: player.GetId(),
			fromServerId: GetServer().GetServerId(),
			cmd:          packet.Command(),
			message:      packet.Message(),
		})
		return true
	} else {
		// 不属于本服务器管理的公会,把消息转发到该公会对应的服务器
		any, err := anypb.New(packet.Message())
		if err != nil {
			logger.Error("anypb err:%v", err)
			return false
		}
		routePacket := &pb.GuildRoutePlayerMessageReq{
			FromPlayerId:   player.GetId(),
			FromGuildId:    guildId,
			FromServerId:   GetServer().GetServerId(),
			FromPlayerName: player.GetName(),
			PacketCommand:  int32(packet.Command()),
			PacketData:     any,
		}
		return GetServerList().SendToServer(routeServerId, PacketCommand(pb.CmdRoute_Cmd_GuildRoutePlayerMessageReq), routePacket)
	}
}

// 公会列表查询
// 这里演示的直接从数据库查询
// 实际项目也可以把列表数据加载到服务器中缓存起来,直接从内存中查询
func OnGuildListReq(player *gameplayer.Player, req *pb.GuildListReq) {
	logger.Debug("OnGuildListReq")
	col := GetGuildDb().(*mongodb.MongoCollection).GetCollection()
	pageSize := int64(10)
	count, err := col.CountDocuments(context.Background(), bson.D{}, nil)
	if err != nil {
		logger.Error("db err:%v", err)
		return
	}
	cursor, dbErr := col.Find(context.Background(), bson.D{}, options.Find().SetSkip(pageSize*int64(req.PageIndex)).SetLimit(pageSize))
	if dbErr != nil {
		logger.Error("db err:%v", dbErr)
		return
	}
	var guildInfos []*pb.GuildInfo
	err = cursor.All(context.Background(), &guildInfos)
	if err != nil {
		logger.Error("db err:%v", err)
		return
	}
	res := &pb.GuildListRes{
		PageIndex:  req.PageIndex,
		PageCount:  int32(math.Ceil(float64(count) / float64(pageSize))),
		GuildInfos: guildInfos,
	}
	gen.SendGuildListRes(player, res)
}

// 创建公会
func OnGuildCreateReq(player *gameplayer.Player, req *pb.GuildCreateReq) {
	logger.Debug("OnGuildCreateReq")
	playerGuild := player.GetGuild()
	if playerGuild.GetGuildData().GuildId > 0 {
		gen.SendGuildCreateRes(player, &pb.GuildCreateRes{
			Error: "CantCreateGuild",
		})
		return
	}
	newId := util.GenUniqueId()
	newGuildData := &pb.GuildData{
		Id: newId,
		BaseInfo: &pb.GuildInfo{
			Id:          newId,
			Name:        req.Name,
			Intro:       req.Intro,
			MemberCount: 1,
		},
		Members: make(map[int64]*pb.GuildMemberData),
	}
	newGuildData.Members[player.GetId()] = &pb.GuildMemberData{
		Id:       player.GetId(),
		Name:     player.GetName(),
		Position: int32(pb.GuildPosition_Leader),
	}
	dbErr, isDuplicateName := GetGuildDb().InsertEntity(newGuildData.Id, newGuildData)
	if dbErr != nil {
		logger.Error("OnGuildCreateReq dbErr:%v", dbErr)
		gen.SendGuildCreateRes(player, &pb.GuildCreateRes{
			Error: "DbError",
		})
		return
	}
	if isDuplicateName {
		gen.SendGuildCreateRes(player, &pb.GuildCreateRes{
			Error: "DuplicateName",
		})
		return
	}
	playerGuild.SetGuildId(newGuildData.Id)
	gen.SendGuildCreateRes(player, &pb.GuildCreateRes{
		Id:   newGuildData.Id,
		Name: newGuildData.BaseInfo.Name,
	})
	logger.Debug("create guild:%v %v", newGuildData.Id, newGuildData.BaseInfo.Name)
}
