package game

import (
	"context"
	"errors"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/network"
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

func (p *Player) GetGuild() *Guild {
	return p.GetComponentByName(ComponentNameGuild).(*Guild)
}

func (g *Guild) GetGuildData() *pb.PlayerGuildData {
	return g.Data
}

func (g *Guild) SyncDataToClient() {
	g.GetPlayer().Send(&pb.GuildSync{
		Data: g.Data,
	})
}

func (g *Guild) SetGuildId(guildId int64) {
	g.Data.GuildId = guildId
	g.SetDirty()
	logger.Debug("%v SetGuildId %v", g.GetPlayerId(), guildId)
}

// 查询公会列表
func (g *Guild) OnGuildListReq(req *pb.GuildListReq) (*pb.GuildListRes, error) {
	logger.Debug("OnGuildListReq")
	guildDb := db.GetGuildDb()
	col := guildDb.(*gentity.MongoCollection).GetCollection()
	pageSize := int64(10)
	count, err := col.CountDocuments(context.Background(), bson.D{}, nil)
	if err != nil {
		logger.Error("db err:%v", err)
		return nil, errors.New("DbError")
	}
	cursor, dbErr := col.Find(context.Background(), bson.D{}, options.Find().SetSkip(pageSize*int64(req.PageIndex)).SetLimit(pageSize))
	if dbErr != nil {
		logger.Error("db err:%v", dbErr)
		return nil, errors.New("DbError")
	}
	type guildBaseInfo struct {
		BaseInfo *pb.GuildInfo `json:"baseinfo"`
	}
	var guildInfos []*guildBaseInfo
	err = cursor.All(context.Background(), &guildInfos)
	if err != nil {
		logger.Error("db err:%v", err)
		return nil, errors.New("DbError")
	}
	res := &pb.GuildListRes{
		PageIndex:  req.PageIndex,
		PageCount:  int32(math.Ceil(float64(count) / float64(pageSize))),
		GuildInfos: make([]*pb.GuildInfo, len(guildInfos), len(guildInfos)),
	}
	for i, info := range guildInfos {
		res.GuildInfos[i] = info.BaseInfo
	}
	return res, nil
}

// 创建公会
func (g *Guild) OnGuildCreateReq(req *pb.GuildCreateReq) (*pb.GuildCreateRes, error) {
	slog.Debug("OnGuildCreateReq")
	player := g.GetPlayer()
	if g.Data.GuildId > 0 {
		return nil, errors.New("AlreadyHaveGuild")
	}
	// NOTE:如果玩家之前已经提交了一个加入其他联盟的请求,玩家又自己创建联盟
	// 其他联盟的管理员又接受了该玩家的加入请求,如何防止该玩家同时存在于2个联盟?
	// 利用mongodb加一个类似原子锁的操作
	newGuildIdValue, err := db.GetKvDb().Inc(db.GuildIdKeyName, int64(1), true)
	if err != nil {
		logger.Error("OnGuildCreateReq err:%v", err)
		return nil, errors.New("IdError")
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
		return nil, errors.New("DbError")
	}
	if isDuplicateName {
		return nil, errors.New("DuplicateName")
	}
	// 利用mongodb的原子操作,来防止该玩家同时加入多个公会
	if !AtomicSetGuildId(g.GetPlayerId(), newGuildData.Id, 0) {
		db.GetGuildDb().DeleteEntity(newGuildData.Id)
		return nil, errors.New("ConcurrentError")
	}
	g.SetGuildId(newGuildData.Id)
	logger.Debug("create guild:%v %v", newGuildData.Id, newGuildData.BaseInfo.Name)
	return &pb.GuildCreateRes{
		Id:   newGuildData.Id,
		Name: newGuildData.BaseInfo.Name,
	}, nil
}

// 加入公会请求
func (g *Guild) OnGuildJoinReq(req *pb.GuildJoinReq) (*pb.GuildJoinRes, error) {
	if g.Data.GuildId > 0 {
		return nil, errors.New("AlreadyHaveGuild")
	}
	// 向公会所在的服务器发rpc请求
	reply := new(pb.GuildJoinRes)
	err := g.RouteRpcToTargetGuild(req.Id, req, reply)
	return reply, err
}

// 公会管理员处理申请者的入会申请
func (g *Guild) OnGuildJoinAgreeReq(req *pb.GuildJoinAgreeReq) (*pb.GuildJoinAgreeRes, error) {
	if g.Data.GuildId == 0 {
		return nil, errors.New("not a guild member")
	}
	// 向公会所在的服务器发rpc请求
	reply := new(pb.GuildJoinAgreeRes)
	err := g.RouteRpcToSelfGuild(req, reply)
	return reply, err
}

// 自己的入会申请的操作结果
//
//	这种格式写的函数可以自动注册非客户端的消息回调
func (g *Guild) HandleGuildJoinReqOpResult(msg *pb.GuildJoinReqOpResult) {
	logger.Debug("HandleGuildJoinReqOpResult:%v", msg)
	if msg.Error == "" && msg.IsAgree {
		// 利用mongodb的原子操作,来防止该玩家同时加入多个公会
		if !AtomicSetGuildId(g.GetPlayerId(), msg.GuildId, 0) {
			msg.Error = "ConcurrentError"
			g.GetPlayer().Send(msg)
			return
		}
		g.SetGuildId(msg.GuildId)
	}
	g.GetPlayer().Send(msg)
}

// 公会成员的客户端的请求消息路由到自己的公会所在服务器
func (g *Guild) RoutePacketToGuild(cmd gnet.PacketCommand, message proto.Message) bool {
	slog.Debug("RoutePacketToGuild", "cmd", cmd, "playerId", g.GetPlayerId(), "guildId", g.Data.GuildId)
	// 转换成给公会服务的路由消息,附带上玩家信息
	routePacket := internal.PacketToGuildRoutePacket(g.GetPlayer().GetId(), g.GetPlayer().GetName(),
		gnet.NewProtoPacketEx(cmd, message), g.Data.GuildId)
	return internal.GetServerList().SendPacket(internal.RouteGuildServerId(g.Data.GuildId), routePacket)
}

// 客户端的请求消息路由到目标公会所在服务器,并阻塞等待返回结果
func (g *Guild) RouteRpcToTargetGuild(targetGuildId int64, message proto.Message, reply proto.Message) error {
	// 转换成给公会服务的路由消息,附带上玩家信息
	routePacket := internal.PacketToGuildRoutePacket(g.GetPlayer().GetId(), g.GetPlayer().GetName(),
		network.NewPacket(message), targetGuildId)
	toServerId := internal.RouteGuildServerId(targetGuildId)
	slog.Debug("RouteRpcToTargetGuild", "playerId", g.GetPlayerId(), "guildId", targetGuildId, "toServerId", toServerId, "req", proto.MessageName(message))
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
func (g *Guild) RouteRpcToSelfGuild(message proto.Message, reply proto.Message) error {
	slog.Debug("RouteRpcToSelfGuild", "playerId", g.GetPlayerId(), "guildId", g.Data.GuildId, "req", proto.MessageName(message))
	return g.RouteRpcToTargetGuild(g.Data.GuildId, message, reply)
}

// 查看自己公会的信息
func (g *Guild) OnGuildDataViewReq(req *pb.GuildDataViewReq) (*pb.GuildDataViewRes, error) {
	if g.Data.GuildId == 0 {
		return nil, errors.New("not a guild member")
	}
	reply := new(pb.GuildDataViewRes)
	err := g.RouteRpcToSelfGuild(req, reply)
	return reply, err
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
