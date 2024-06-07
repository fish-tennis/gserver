package social

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/game"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
)

var _ gentity.RoutineEntity = (*Guild)(nil)

// 公会
type Guild struct {
	gentity.BaseRoutineEntity

	BaseInfo     *GuildBaseInfo     `child:"baseinfo"`
	Members      *GuildMembers      `child:"members"`
	JoinRequests *GuildJoinRequests `child:"joinrequests"`
}

// requestPacket->route to guild->convert packet to guildMessage-->guild.PushMessage
type GuildMessage struct {
	fromPlayerId   int64
	fromServerId   int32
	fromPlayerName string
	cmd            PacketCommand
	message        proto.Message
	srcPacket      Packet     // 来源packet
	srcConnection  Connection // 来源连接
	//putReplyFn     func(reply Packet)
}

func NewGuild(guildData *pb.GuildLoadData) *Guild {
	guild := &Guild{
		BaseRoutineEntity: *gentity.NewRoutineEntity(32),
	}
	guild.Id = guildData.Id
	guild.BaseInfo = NewGuildBaseInfo(guild, guildData.BaseInfo)
	guild.AddComponent(guild.BaseInfo, nil)
	guild.Members = NewGuildMembers(guild, guildData.Members)
	guild.AddComponent(guild.Members, nil)
	guild.JoinRequests = NewGuildJoinRequests(guild)
	guild.AddComponent(guild.JoinRequests, guildData.JoinRequests)
	return guild
}

func (this *Guild) processMessage(guildMessage *GuildMessage) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("recover:%v", err)
			logger.LogStack()
		}
	}()
	// TODO: handler map
	switch v := guildMessage.message.(type) {
	case *pb.GuildDataViewReq:
		logger.Debug("%v", v)
		this.OnGuildDataViewReq(guildMessage, v)
	case *pb.GuildJoinReq:
		logger.Debug("%v", v)
		this.OnGuildJoinReq(guildMessage, v)
	case *pb.GuildJoinAgreeReq:
		logger.Debug("%v", v)
		this.OnGuildJoinAgreeReq(guildMessage, v)
	default:
		if pm, ok := guildMessage.message.(proto.Message); ok {
			logger.Debug("ignore %v", proto.MessageName(pm))
		} else {
			logger.Debug("ignore %v", v)
		}
	}
}

func (this *Guild) GetMember(playerId int64) *pb.GuildMemberData {
	return this.Members.Get(playerId)
}

// 路由玩家消息
// this server -> other server -> player
func (this *Guild) RoutePlayerPacket(guildMessage *GuildMessage, cmd PacketCommand, message proto.Message, opts ...game.RouteOption) {
	routePacket := NewProtoPacketEx(cmd, message)
	if protoPacket, ok := guildMessage.srcPacket.(*ProtoPacket); ok {
		routePacket.SetRpcCallId(protoPacket.RpcCallId())
	}
	newOpts := make([]game.RouteOption, len(opts)+1)
	// 回消息时,使用来源连接,才能让rpc调用方收到结果
	newOpts[0] = game.WithConnection(guildMessage.srcConnection)
	for i, opt := range opts {
		newOpts[i+1] = opt
	}
	game.RoutePlayerPacket(guildMessage.fromPlayerId, routePacket, newOpts...)
}

// 路由玩家消息,直接发给客户端
// this server -> other server -> client
func (this *Guild) RouteClientPacket(guildMessage *GuildMessage, cmd PacketCommand, message proto.Message) {
	game.RoutePlayerPacket(guildMessage.fromPlayerId, NewProtoPacketEx(cmd, message),
		game.WithDirectSendClient(), game.WithConnection(guildMessage.srcConnection))
}

// 广播公会消息
// this server -> other server -> player
func (this *Guild) BroadcastPlayerPacket(cmd PacketCommand, message proto.Message) {
	for _, member := range this.Members.Data {
		game.RoutePlayerPacket(member.Id, NewProtoPacketEx(cmd, message))
	}
}

// 广播公会消息,直接发给客户端
// this server -> other server -> client
func (this *Guild) BroadcastClientPacket(cmd PacketCommand, message proto.Message) {
	for _, member := range this.Members.Data {
		game.RoutePlayerPacket(member.Id, NewProtoPacketEx(cmd, message), game.WithDirectSendClient())
	}
}

// 加入公会请求
func (this *Guild) OnGuildJoinReq(guildMessage *GuildMessage, req *pb.GuildJoinReq) {
	errStr := ""
	defer this.RoutePlayerPacket(guildMessage, PacketCommand(pb.CmdGuild_Cmd_GuildJoinRes), &pb.GuildJoinRes{
		Error: errStr,
		Id:    this.GetId(),
	})
	if this.GetMember(guildMessage.fromPlayerId) != nil {
		errStr = "already a member"
		return
	}
	if this.JoinRequests.Get(guildMessage.fromPlayerId) != nil {
		errStr = "already have a join request"
		return
	}
	this.JoinRequests.Add(&pb.GuildJoinRequest{
		PlayerId:     guildMessage.fromPlayerId,
		PlayerName:   guildMessage.fromPlayerName,
		TimestampSec: int32(util.GetCurrentTimeStamp()),
	})
	// 广播公会成员
	this.BroadcastClientPacket(PacketCommand(pb.CmdGuild_Cmd_GuildJoinReqTip), &pb.GuildJoinReqTip{
		PlayerId:   guildMessage.fromPlayerId,
		PlayerName: guildMessage.fromPlayerName,
	})
	logger.Debug("OnGuildJoinReq %v %v", this.GetId(), guildMessage.fromPlayerId)
}

// 公会管理者同意申请人加入公会
func (this *Guild) OnGuildJoinAgreeReq(guildMessage *GuildMessage, req *pb.GuildJoinAgreeReq) {
	errStr := ""
	// 返回操作结果
	defer this.RoutePlayerPacket(guildMessage, PacketCommand(pb.CmdGuild_Cmd_GuildJoinAgreeRes), &pb.GuildJoinAgreeRes{
		Error:           errStr,
		GuildId:         this.GetId(),
		ManagerPlayerId: guildMessage.fromPlayerId,
		JoinPlayerId:    req.JoinPlayerId,
		IsAgree:         req.IsAgree,
	}, game.WithSaveDb())
	member := this.GetMember(guildMessage.fromPlayerId)
	if member == nil {
		errStr = "already joined"
		return
	}
	if member.Position < int32(pb.GuildPosition_Manager) {
		errStr = "not a manager"
		return
	}
	joinRequest := this.JoinRequests.Get(req.JoinPlayerId)
	if joinRequest == nil {
		errStr = "no joinRequest"
		return
	}
	if req.IsAgree {
		// TODO:如果玩家之前已经提交了一个加入其他联盟的请求,玩家又自己创建联盟
		// 其他联盟的管理员又接受了该玩家的加入请求,如何防止该玩家同时存在于2个联盟?
		// 利用mongodb加一个类似原子锁的操作?
		this.Members.Add(&pb.GuildMemberData{
			Id:       joinRequest.PlayerId,
			Name:     joinRequest.PlayerName,
			Position: int32(pb.GuildPosition_Member),
		})
		this.BaseInfo.SetMemberCount(int32(len(this.Members.Data)))
	}
	this.JoinRequests.Remove(req.JoinPlayerId)
}

// 查看公会数据
func (this *Guild) OnGuildDataViewReq(guildMessage *GuildMessage, req *pb.GuildDataViewReq) {
	if this.GetMember(guildMessage.fromPlayerId) == nil {
		return
	}
	this.RouteClientPacket(guildMessage, PacketCommand(pb.CmdGuild_Cmd_GuildDataViewRes), &pb.GuildDataViewRes{
		GuildData: &pb.GuildData{
			Id:           this.GetId(),
			BaseInfo:     this.BaseInfo.Data,
			Members:      this.Members.Data,
			JoinRequests: this.JoinRequests.Data,
		},
	})
}
