package game

import (
	"context"
	"errors"
	"log/slog"
	"math"

	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"google.golang.org/protobuf/proto"
)

const (
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
	slog.Debug("Guild.SetGuildId", "playerId", g.GetPlayerId(), "guildId", guildId)
}

// 查询公会列表
func (g *Guild) OnGuildListReq(req *pb.GuildListReq) (*pb.GuildListRes, error) {
	slog.Debug("Guild.OnGuildListReq")
	// 校验 PageIndex,防止负值产生负 skip
	if req.PageIndex < 0 {
		return nil, errors.New("PageIndexError")
	}
	guildDb := db.GetGuildDb()
	col := guildDb.(*gentity.MongoCollection).GetCollection()
	pageSize := int64(10)
	count, err := col.CountDocuments(context.Background(), bson.D{}, nil)
	if err != nil {
		slog.Error("Guild.OnGuildListReq: db error", "error", err)
		return nil, errors.New("DbError")
	}
	// 按_id排序保证分页稳定:分片集群上无sort的skip/limit返回顺序不确定,
	// 翻页会出现重复/漏公会;单机环境下排序也是分页稳定性的必要条件
	cursor, dbErr := col.Find(context.Background(), bson.D{},
		options.Find().SetSort(bson.D{{Key: db.UniqueIdName, Value: 1}}).
			SetSkip(pageSize*int64(req.PageIndex)).SetLimit(pageSize))
	if dbErr != nil {
		slog.Error("Guild.OnGuildListReq: db error", "error", dbErr)
		return nil, errors.New("DbError")
	}
	defer cursor.Close(context.Background())
	type guildBaseInfo struct {
		BaseInfo *pb.GuildInfo `json:"baseinfo"`
	}
	var guildInfos []*guildBaseInfo
	err = cursor.All(context.Background(), &guildInfos)
	if err != nil {
		slog.Error("Guild.OnGuildListReq: db error", "error", err)
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
	slog.Debug("Guild.OnGuildCreateReq")
	player := g.GetPlayer()
	if g.Data.GuildId > 0 {
		return nil, errors.New("AlreadyHaveGuild")
	}
	// NOTE:如果玩家之前已经提交了一个加入其他联盟的请求,玩家又自己创建联盟
	// 其他联盟的管理员又接受了该玩家的加入请求,如何防止该玩家同时存在于2个联盟?
	// 利用mongodb加一个类似原子锁的操作
	newGuildIdValue, err := db.GetKvDb().Inc(db.GuildIdKeyName, int64(1), true)
	if err != nil {
		slog.Error("Guild.OnGuildCreateReq: id error", "error", err)
		return nil, errors.New("IdError")
	}
	// BSON数值可能是int32/int64/float64等类型,用安全转换
	var newGuildId int64
	switch idVal := newGuildIdValue.(type) {
	case int64:
		newGuildId = idVal
	case int32:
		newGuildId = int64(idVal)
	case float64:
		newGuildId = int64(idVal)
	default:
		slog.Error("OnGuildCreateReq invalid guildId type", "newGuildIdValue", newGuildIdValue)
		return nil, errors.New("IdError")
	}
	if newGuildId <= 0 {
		slog.Error("OnGuildCreateReq invalid guildId", "newGuildIdValue", newGuildIdValue)
		return nil, errors.New("IdError")
	}
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
		slog.Error("Guild.OnGuildCreateReq: db error", "error", dbErr)
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
	slog.Debug("Guild.OnGuildCreateReq: created", "guildId", newGuildData.Id, "name", newGuildData.BaseInfo.Name)
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
	slog.Debug("Guild.HandleGuildJoinReqOpResult", "msg", msg)
	if msg.Error == "" && msg.IsAgree {
		// 公会服务器已通过 AtomicSetGuildId 原子写入 DB,玩家端只需更新本地内存状态
		// 不再重复调用 AtomicSetGuildId,否则会因 DB 中 guildId 已设置而 filter(old==0) 不匹配导致失败
		g.SetGuildId(msg.GuildId)
	}
	g.GetPlayer().Send(msg)
}

// 公会成员的客户端的请求消息路由到自己的公会所在服务器
func (g *Guild) RoutePacketToGuild(cmd gnet.PacketCommand, message proto.Message) bool {
	slog.Debug("Guild.RoutePacketToGuild", "cmd", cmd, "playerId", g.GetPlayerId(), "guildId", g.Data.GuildId)
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
	slog.Debug("Guild.RouteRpcToTargetGuild", "playerId", g.GetPlayerId(), "guildId", targetGuildId, "toServerId", toServerId, "req", proto.MessageName(message))
	routePlayerMessage := new(pb.RoutePlayerMessage)
	err := internal.GetServerList().Rpc(toServerId, routePacket, routePlayerMessage)
	if err != nil {
		slog.Error("Guild.RouteRpcToTargetGuild error", "toServerId", toServerId, "error", err)
	}
	if err == nil {
		if routePlayerMessage.Error != "" {
			slog.Error("Guild.RouteRpcToTargetGuild error", "toServerId", toServerId, "error", routePlayerMessage.Error)
			return errors.New(routePlayerMessage.Error)
		}
		err = routePlayerMessage.PacketData.UnmarshalTo(reply)
		if err != nil {
			slog.Error("Guild.RouteRpcToTargetGuild: parse reply error", "error", err, "reply", reply,
				"res", string(routePlayerMessage.PacketData.MessageName().Name()))
		}
	}
	return err
}

// 公会成员的客户端的请求消息路由到自己的公会所在服务器,并阻塞等待返回结果
func (g *Guild) RouteRpcToSelfGuild(message proto.Message, reply proto.Message) error {
	slog.Debug("Guild.RouteRpcToSelfGuild", "playerId", g.GetPlayerId(), "guildId", g.Data.GuildId, "req", proto.MessageName(message))
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
	// player库通过RegisterPlayerDb注册后,GetEntityDb返回的是*gentity.MongoCollectionPlayer(内嵌MongoCollection)
	// 直接断言*gentity.MongoCollection会panic,这里兼容两种类型
	var col *gentity.MongoCollection
	switch entityDb := db.GetDbMgr().GetEntityDb(db.PlayerDbName).(type) {
	case *gentity.MongoCollectionPlayer:
		col = &entityDb.MongoCollection
	case *gentity.MongoCollection:
		col = entityDb
	default:
		slog.Error("AtomicSetGuildId: unsupported player db type", "playerId", playerId, "guildId", guildId)
		return false
	}
	// NOTE: 明文保存的proto字段,字段名会被mongodb自动转为小写 Q:有办法解决吗?
	// 所以这里的guildid用全小写
	fieldKey := "Guild.guildid"
	// 构建filter:玩家id + 当前guildId必须匹配oldGuildId(或为0表示新加入)
	var filterGuildIds []any
	if oldGuildId != 0 {
		filterGuildIds = []any{oldGuildId}
	} else {
		// oldGuildId=0 表示新加入,检查当前guildId必须为0(未加入任何公会)
		filterGuildIds = []any{int64(0)}
	}
	filter := bson.D{
		{db.UniqueIdName, playerId},
		{fieldKey, bson.D{{"$in", filterGuildIds}}},
	}
	result := col.GetCollection().FindOneAndUpdate(context.Background(),
		filter,
		bson.D{{"$set", bson.D{{fieldKey, guildId}}}})
	err := result.Err()
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			// filter 不匹配:真正的并发冲突
			slog.Debug("AtomicSetGuildId concurrent", "playerId", playerId, "guildId", guildId, "oldGuildId", oldGuildId)
			return false
		}
		// DB 异常(网络分区、超时等),不应误判为并发冲突
		slog.Error("AtomicSetGuildId db error", "playerId", playerId, "guildId", guildId, "err", err)
		return false
	}
	return true
}
