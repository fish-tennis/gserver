package social

import (
	"context"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/db/mongodb"
	"github.com/fish-tennis/gserver/gameplayer"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/protobuf/types/known/anypb"
	"math"
	"sync"
)

var (
	_guildMap = make(map[int64]*Guild)
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
	guildData := &pb.GuildData{
		Id: guildId,
	}
	exist,err := GetGuildDb().FindEntityById(guildId, guildData)
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
	if existGuild,ok := _guildMap[guildId]; ok {
		return existGuild
	}
	_guildMap[guildId] = guild
	guild.StartProcessRoutine()
	return guild
}

// 公会db接口
func GetGuildDb() db.EntityDb {
	return db.GetDbMgr().GetEntityDb("guild")
}

// 根据公会id路由到对应的服务器id
func GuildRoute(guildId int64) int32 {
	servers := internal.GetServerList().GetServersByType("game")
	// 这里只演示了最简单的路由方式
	index := guildId % int64(len(servers))
	return servers[index].ServerId
}

// 根据公会id路由消息
func GuildRouteReqPacket(player *gameplayer.Player, guildId int64, packet *ProtoPacket) bool {
	routeServerId := GuildRoute(guildId)
	logger.Debug("GuildRouteReqPacket %v -> %v", guildId, routeServerId)
	if routeServerId <= 0 {
		return false
	}
	// 属于本服务器管理的公会
	if internal.GetServer().GetServerId() == routeServerId {
		// 先从内存中查找
		guild := GetGuildById(guildId)
		if guild == nil {
			// 再到数据库加载
			guild = LoadGuild(guildId)
			if guild == nil {
				logger.Error("not find guild:%v", guild)
				return false
			}
		}
		guild.PushMessage(&GuildMessage{
			fromPlayerId: player.GetId(),
			fromServerId: internal.GetServer().GetServerId(),
			cmd: packet.Command(),
			message: packet.Message(),
		})
		return true
	} else {
		// 不属于本服务器管理的公会,把消息转发到该公会对应的服务器
		any,err := anypb.New(packet.Message())
		if err != nil {
			logger.Error("anypb err:%v", err)
			return false
		}
		routePacket := &pb.GuildRoutePlayerMessageReq{
			FromPlayerId: player.GetId(),
			FromGuildId: guildId,
			FromServerId: internal.GetServer().GetServerId(),
			PacketCommand: int32(packet.Command()),
			PacketData: any,
		}
		return internal.GetServerList().SendToServer(routeServerId, PacketCommand(pb.CmdRoute_Cmd_GuildRoutePlayerMessageReq), routePacket)
	}
}

// 公会列表查询
// 这里演示的直接从数据库查询
// 实际项目也可以把列表数据加载到服务器中缓存起来,直接从内存中查询
func OnGuildListReq(player *gameplayer.Player, req *pb.GuildListReq) {
	logger.Debug("OnGuildListReq")
	col := GetGuildDb().(*mongodb.MongoCollection).GetCollection()
	pageSize := int64(10)
	count,err := col.CountDocuments(context.Background(), bson.D{},nil)
	if err != nil {
		logger.Error("db err:%v", err)
		return
	}
	cursor,err := col.Find(context.Background(), bson.D{}, options.Find().SetSkip(pageSize*int64(req.PageIndex)).SetLimit(pageSize))
	if err != nil {
		logger.Error("db err:%v", err)
		return
	}
	var guildInfos []*pb.GuildInfo
	err = cursor.All(context.Background(), &guildInfos)
	if err != nil {
		logger.Error("db err:%v", err)
		return
	}
	res := &pb.GuildListRes{
		PageIndex: req.PageIndex,
		PageCount: int32(math.Ceil(float64(count)/float64(pageSize))),
		GuildInfos: guildInfos,
	}
	player.SendGuildListRes(res)
}

// 创建公会
func OnGuildCreateReq(player *gameplayer.Player, req *pb.GuildCreateReq) {
	logger.Debug("OnGuildCreateReq")
	playerGuild := player.GetGuild()
	if playerGuild.GetGuildData().GuildId > 0 {
		player.SendGuildCreateRes(&pb.GuildCreateRes{
			Error: "CantCreateGuild",
		})
		return
	}
	newId := util.GenUniqueId()
	newGuildData := &pb.GuildData{
		Id: newId,
		BaseInfo: &pb.GuildInfo{
			Id: newId,
			Name: req.Name,
		},
		Members: make(map[int64]*pb.GuildMemberData),
	}
	newGuildData.Members[player.GetId()] = &pb.GuildMemberData{
		Id: player.GetId(),
		Name: player.GetName(),
		Position: int32(pb.GuildPosition_Leader),
	}
	dbErr,isDuplicateName := GetGuildDb().InsertEntity(newGuildData.Id, newGuildData)
	if dbErr != nil {
		logger.Error("OnGuildCreateReq dbErr:%v", dbErr)
		player.SendGuildCreateRes(&pb.GuildCreateRes{
			Error: "DbError",
		})
		return
	}
	if isDuplicateName {
		player.SendGuildCreateRes(&pb.GuildCreateRes{
			Error: "DuplicateName",
		})
		return
	}
	playerGuild.SetGuildId(newGuildData.Id)
	player.SendGuildCreateRes(&pb.GuildCreateRes{
		Id:   newGuildData.Id,
		Name: newGuildData.BaseInfo.Name,
	})
	logger.Debug("create guild:%v %v", newGuildData.Id, newGuildData.BaseInfo.Name)
}