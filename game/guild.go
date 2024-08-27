package game

import (
	"context"
	"errors"
	"fmt"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/db"
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
	_playerComponentRegister.Register(ComponentNameGuild, 100, func(player *Player, _ any) gentity.Component {
		return &Guild{
			PlayerDataComponent: *NewPlayerDataComponent(player, ComponentNameGuild),
			Data: &pb.PlayerGuildData{
				GuildId: 0,
			},
		}
	})
}

// 玩家的公会模块
type Guild struct {
	PlayerDataComponent
	// 这里使用明文方式保存数据,以便使用mongodb语句直接进行操作,如AtomicSetGuildId函数
	Data *pb.PlayerGuildData `db:"plain"`
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
func (this *Guild) OnGuildListReq(req *pb.GuildListReq) {
	logger.Debug("OnGuildListReq")
	guildDb := db.GetGuildDb()
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
	this.GetPlayer().Send(res)
}

// 创建公会
func (this *Guild) OnGuildCreateReq(req *pb.GuildCreateReq) {
	logger.Debug("OnGuildCreateReq")
	player := this.GetPlayer()
	if this.Data.GuildId > 0 {
		this.GetPlayer().Send(&pb.GuildCreateRes{
			Error: "AlreadyHaveGuild",
		})
		return
	}
	// TODO:如果玩家之前已经提交了一个加入其他联盟的请求,玩家又自己创建联盟
	// 其他联盟的管理员又接受了该玩家的加入请求,如何防止该玩家同时存在于2个联盟?
	// 利用mongodb加一个类似原子锁的操作?
	newGuildIdValue, err := db.GetKvDb().Inc(db.GuildIdKeyName, int64(1), true)
	if err != nil {
		this.GetPlayer().Send(&pb.GuildCreateRes{
			Error: "IdError",
		})
		logger.Error("OnGuildCreateReq err:%v", err)
		return
	}
	newGuildId := newGuildIdValue.(int64)
	newGuildData := &pb.GuildData{
		Id: newGuildId,
		BaseInfo: &pb.GuildInfo{
			Id:          newGuildId,
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
	guildDb := db.GetGuildDb()
	saveData := map[string]any{
		db.UniqueIdName: newGuildData.Id, // mongodb _id特殊处理
		"Id":            newGuildData.Id,
		"BaseInfo":      newGuildData.BaseInfo,
		"Members":       newGuildData.Members,
	}
	dbErr, isDuplicateName := guildDb.InsertEntity(newGuildData.Id, saveData)
	if dbErr != nil {
		logger.Error("OnGuildCreateReq dbErr:%v", dbErr)
		player.Send(&pb.GuildCreateRes{
			Error: "DbError",
		})
		return
	}
	if isDuplicateName {
		player.Send(&pb.GuildCreateRes{
			Error: "DuplicateName",
		})
		return
	}
	// 利用mongodb的原子操作,来防止该玩家同时加入多个公会
	if !AtomicSetGuildId(this.GetPlayerId(), newGuildData.Id, 0) {
		db.GetGuildDb().DeleteEntity(newGuildData.Id)
		player.Send(&pb.GuildCreateRes{
			Error: "ConcurrentError",
		})
		return
	}
	this.SetGuildId(newGuildData.Id)
	player.Send(&pb.GuildCreateRes{
		Id:   newGuildData.Id,
		Name: newGuildData.BaseInfo.Name,
	})
	logger.Debug("create guild:%v %v", newGuildData.Id, newGuildData.BaseInfo.Name)
}

// 加入公会请求
func (this *Guild) OnGuildJoinReq(req *pb.GuildJoinReq) {
	if this.Data.GuildId > 0 {
		this.player.SendErrorRes(gnet.PacketCommand(pb.CmdClient_Cmd_GuildJoinReq), "AlreadyHaveGuild")
		return
	}
	// 向公会所在的服务器发rpc请求
	reply := new(pb.GuildJoinRes)
	err := this.RouteRpcToTargetGuild(req.Id, gnet.PacketCommand(pb.CmdClient_Cmd_GuildJoinReq), req, reply)
	if err != nil {
		this.player.SendErrorRes(gnet.PacketCommand(pb.CmdClient_Cmd_GuildJoinReq), fmt.Sprintf("server internal error:%v", err.Error()))
		return
	}
	slog.Debug("OnGuildJoinReq reply", "reply", reply)
	this.GetPlayer().Send(reply)
}

// 公会管理员处理申请者的入会申请
func (this *Guild) OnGuildJoinAgreeReq(req *pb.GuildJoinAgreeReq) {
	if this.Data.GuildId == 0 {
		this.player.SendErrorRes(gnet.PacketCommand(pb.CmdClient_Cmd_GuildJoinAgreeReq), "not a guild member")
		return
	}
	// 向公会所在的服务器发rpc请求
	reply := new(pb.GuildJoinAgreeRes)
	err := this.RouteRpcToSelfGuild(gnet.PacketCommand(pb.CmdClient_Cmd_GuildJoinAgreeReq), req, reply)
	if err != nil {
		this.player.SendErrorRes(gnet.PacketCommand(pb.CmdClient_Cmd_GuildJoinAgreeReq), fmt.Sprintf("server internal error:%v", err.Error()))
		return
	}
	slog.Debug("OnGuildJoinAgreeReq reply", "reply", reply)
	this.GetPlayer().Send(reply)
}

// 自己的入会申请的操作结果
//
//	这种格式写的函数可以自动注册非客户端的消息回调
func (this *Guild) HandleGuildJoinReqOpResult(msg *pb.GuildJoinReqOpResult) {
	logger.Debug("HandleGuildJoinReqOpResult:%v", msg)
	if msg.Error == "" && msg.IsAgree {
		// 利用mongodb的原子操作,来防止该玩家同时加入多个公会
		if !AtomicSetGuildId(this.GetPlayerId(), msg.GuildId, 0) {
			msg.Error = "ConcurrentError"
			this.GetPlayer().Send(msg)
			return
		}
		this.SetGuildId(msg.GuildId)
	}
	this.GetPlayer().Send(msg)
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
		if routePlayerMessage.Error != "" {
			slog.Error("RouteRpcToTargetGuildErr", "toServerId", toServerId, "err", routePlayerMessage.Error)
			return errors.New(routePlayerMessage.Error)
		}
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
func (this *Guild) OnGuildDataViewReq(req *pb.GuildDataViewReq) {
	if this.Data.GuildId == 0 {
		this.GetPlayer().SendErrorRes(gnet.PacketCommand(pb.CmdClient_Cmd_GuildDataViewReq), "not a guild member")
		return
	}
	reply := new(pb.GuildDataViewRes)
	err := this.RouteRpcToSelfGuild(gnet.PacketCommand(pb.CmdClient_Cmd_GuildDataViewReq), req, reply)
	if err != nil {
		this.GetPlayer().SendErrorRes(gnet.PacketCommand(pb.CmdClient_Cmd_GuildDataViewReq), fmt.Sprintf("server internal error:%v", err.Error()))
		return
	}
	this.GetPlayer().Send(reply)
}

// mongodb中对玩家公会id进行原子化操作,防止玩家同时存在于多个公会
//
//	比如:
//	step1:玩家向公会A,B发送入会申请
//	step2:公会A,B的管理员同时操作,同意入会申请,如果没有原子化保证,玩家将同时加入到A,B公会
func AtomicSetGuildId(playerId int64, guildId int64, oldGuildId int64) bool {
	col := db.GetPlayerDb().(*gentity.MongoCollectionPlayer)
	// NOTE: 明文保存的proto字段,字段名会被mongodb自动转为小写 Q:有办法解决吗?
	// 所以这里的guildid用全小写
	fieldKey := "Guild.guildid"
	filter := bson.D{
		{db.UniqueIdName, playerId},
		{fieldKey, bson.D{{"$in", []any{int64(0), guildId}}}},
	}
	result := col.GetCollection().FindOneAndUpdate(context.Background(),
		filter,
		bson.D{{"$set", bson.D{{fieldKey, guildId}}}})
	slog.Debug("AtomicSetGuildId", "playerId", playerId, "guildId", guildId, "oldGuildId", oldGuildId, "err", result.Err())
	return result.Err() == nil
}
