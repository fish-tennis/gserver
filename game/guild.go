package game

import (
	"context"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/gen"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"math"
)

const (
	// 组件名
	ComponentNameGuild = "Guild"
)

// 利用go的init进行组件的自动注册
func init() {
	RegisterPlayerComponentCtor(ComponentNameGuild, 100, func(player *Player, playerData *pb.PlayerData) gentity.Component {
		component := &Guild{
			PlayerDataComponent: *NewPlayerDataComponent(player, ComponentNameGuild),
			Data: &pb.PlayerGuildData{
				GuildId: 0,
			},
		}
		gentity.LoadData(component, playerData.GetGuild())
		return component
	})
}

// 玩家的公会模块
type Guild struct {
	PlayerDataComponent
	Data *pb.PlayerGuildData `db:"Guild"`
}

func (this *Player) GetGuild() *Guild {
	return this.GetComponentByName(ComponentNameGuild).(*Guild)
}

func (this *Guild) GetGuildData() *pb.PlayerGuildData {
	return this.Data
}

func (this *Guild) SetGuildId(guildId int64) {
	this.Data.GuildId = guildId
	this.SetDirty()
	logger.Debug("%v SetGuildId %v", this.GetPlayerId(), guildId)
}

// 查询公会列表
func (this *Guild) OnGuildListReq(reqCmd gnet.PacketCommand, req *pb.GuildListReq) {
	logger.Debug("OnGuildListReq")
	guildDb := db.GetDbMgr().GetEntityDb(db.GuildDbName)
	col := guildDb.(*gentity.MongoCollection).GetCollection()
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
	type guildBaseInfo struct {
		BaseInfo *pb.GuildInfo `json:"baseinfo"`
	}
	var guildInfos []*guildBaseInfo
	err = cursor.All(context.Background(), &guildInfos)
	if err != nil {
		logger.Error("db err:%v", err)
		return
	}
	res := &pb.GuildListRes{
		PageIndex:  req.PageIndex,
		PageCount:  int32(math.Ceil(float64(count) / float64(pageSize))),
		GuildInfos: make([]*pb.GuildInfo, len(guildInfos), len(guildInfos)),
	}
	for i, info := range guildInfos {
		res.GuildInfos[i] = info.BaseInfo
	}
	gen.SendGuildListRes(this.GetPlayer(), res)
}

// 创建公会
func (this *Guild) OnGuildCreateReq(reqCmd gnet.PacketCommand, req *pb.GuildCreateReq) {
	logger.Debug("OnGuildCreateReq")
	player := this.GetPlayer()
	if this.Data.GuildId > 0 {
		gen.SendGuildCreateRes(player, &pb.GuildCreateRes{
			Error: "AlreadyHaveGuild",
		})
		return
	}
	// TODO:如果玩家之前已经提交了一个加入其他联盟的请求,玩家又自己创建联盟
	// 其他联盟的管理员又接受了该玩家的加入请求,如何防止该玩家同时存在于2个联盟?
	// 利用mongodb加一个类似原子锁的操作?
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
	guildDb := db.GetDbMgr().GetEntityDb(db.GuildDbName)
	saveData := gentity.ConvertProtoToMap(newGuildData)
	// mongodb _id特殊处理
	saveData["_id"] = newGuildData.Id
	dbErr, isDuplicateName := guildDb.InsertEntity(newGuildData.Id, saveData)
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
	this.SetGuildId(newGuildData.Id)
	gen.SendGuildCreateRes(player, &pb.GuildCreateRes{
		Id:   newGuildData.Id,
		Name: newGuildData.BaseInfo.Name,
	})
	logger.Debug("create guild:%v %v", newGuildData.Id, newGuildData.BaseInfo.Name)
}

// 加入公会请求
func (this *Guild) OnGuildJoinReq(reqCmd gnet.PacketCommand, req *pb.GuildJoinReq) {
	if this.Data.GuildId > 0 {
		this.player.SendErrorRes(reqCmd, "AlreadyHaveGuild")
		return
	}
	// 向公会所在的服务器发rpc请求
	reply := new(pb.GuildJoinRes)
	err := this.RouteRpcToTargetGuild(req.Id, reqCmd, req, reply)
	if err != nil {
		this.player.SendErrorRes(reqCmd, "server internal error")
		return
	}
	slog.Debug("OnGuildJoinReq reply", "reply", reply)
	gen.SendGuildJoinRes(this.GetPlayer(), reply)
}

// 加入联盟的请求的回复结果
//
//	这种格式写的函数可以自动注册非客户端的消息回调
func (this *Guild) HandleGuildJoinAgreeRes(cmd gnet.PacketCommand, msg *pb.GuildJoinAgreeRes) {
	logger.Debug("HandleGuildJoinAgreeRes:%v", msg)
	if msg.IsAgree {
		this.SetGuildId(msg.GuildId)
	}
	this.GetPlayer().Send(cmd, msg)
}

// 公会成员的客户端的请求消息路由到自己的公会所在服务器
func (this *Guild) RoutePacketToGuild(cmd gnet.PacketCommand, message proto.Message) bool {
	slog.Debug("RoutePacketToGuild", "cmd", cmd, "playerId", this.GetPlayerId(), "guildId", this.Data.GuildId)
	// 转换成给公会服务的路由消息,附带上玩家信息
	routePacket := internal.PacketToGuildRoutePacket(this.GetPlayer().GetId(), this.GetPlayer().GetName(),
		gnet.NewProtoPacketEx(cmd, message), this.Data.GuildId)
	return internal.GetServerList().SendPacket(internal.RouteGuildServerId(this.Data.GuildId), routePacket)
}

// 客户端的请求消息路由到目标公会所在服务器,并阻塞等待返回结果
func (this *Guild) RouteRpcToTargetGuild(targetGuildId int64, cmd gnet.PacketCommand, message proto.Message, reply proto.Message) error {
	// 转换成给公会服务的路由消息,附带上玩家信息
	routePacket := internal.PacketToGuildRoutePacket(this.GetPlayer().GetId(), this.GetPlayer().GetName(),
		gnet.NewProtoPacketEx(cmd, message), targetGuildId)
	toServerId := internal.RouteGuildServerId(targetGuildId)
	slog.Debug("RouteRpcToTargetGuild", "cmd", cmd, "playerId", this.GetPlayerId(), "guildId", targetGuildId, "toServerId", toServerId)
	routePlayerMessage := new(pb.RoutePlayerMessage)
	err := internal.GetServerList().Rpc(toServerId, routePacket, routePlayerMessage)
	if err != nil {
		slog.Error("RouteRpcToTargetGuildErr", "toServerId", toServerId, "err", err)
	}
	if err == nil {
		err = routePlayerMessage.PacketData.UnmarshalTo(reply)
		if err != nil {
			slog.Error("ParseReplyErr", "err", err, "reply", reply,
				"res", string(routePlayerMessage.PacketData.MessageName().Name()))
		}
	}
	return err
}

// 公会成员的客户端的请求消息路由到自己的公会所在服务器,并阻塞等待返回结果
func (this *Guild) RouteRpcToSelfGuild(cmd gnet.PacketCommand, message proto.Message, reply proto.Message) error {
	slog.Debug("RouteRpcToSelfGuild", "cmd", cmd, "playerId", this.GetPlayerId(), "guildId", this.Data.GuildId)
	return this.RouteRpcToTargetGuild(this.Data.GuildId, cmd, message, reply)
}

// 查看自己公会的信息
func (this *Guild) OnGuildDataViewReq(reqCmd gnet.PacketCommand, req *pb.GuildDataViewReq) {
	this.RoutePacketToGuild(reqCmd, req)
}
