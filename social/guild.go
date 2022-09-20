package social

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/game"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"time"
)

var _ gentity.Entity = (*Guild)(nil)

// 公会
type Guild struct {
	gentity.RoutineEntity
	
	BaseInfo     *GuildBaseInfo     `child:"baseinfo"`
	Members      *GuildMembers      `child:"members"`
	JoinRequests *GuildJoinRequests `child:"joinrequests"`
}

type GuildMessage struct {
	fromPlayerId   int64
	fromServerId   int32
	fromPlayerName string
	cmd            PacketCommand
	message        proto.Message
}

func NewGuild(guildData *pb.GuildLoadData) *Guild {
	guild := &Guild{
		RoutineEntity: *gentity.NewRoutineEntity(32),
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

func (this *Guild) PushMessage(guildMessage *GuildMessage) {
	logger.Debug("PushMessage %v", guildMessage)
	this.RoutineEntity.PushMessage(guildMessage)
	//this.messages <- guildMessage
}

// 开启消息处理协程
func (this *Guild) RunProcessRoutine() bool {
	logger.Debug("guild RunProcessRoutine %v", this.GetId())
	return this.RoutineEntity.RunProcessRoutine(&gentity.RoutineEntityRoutineArgs{
		InitFunc: func(routineEntity gentity.Entity) bool {
			// redis实现的分布式锁,保证同一个公会的逻辑处理协程只会在一个服务器上
			if !guildServerLock(this.GetId()) {
				return false
			}
			return true
		},
		EndFunc: func(routineEntity gentity.Entity) {
			// 协程结束的时候,分布式锁UnLock
			guildServerUnlock(this.GetId())
		},
		ProcessMessageFunc: func(routineEntity gentity.Entity, message interface{}) {
			this.processMessage(message.(*GuildMessage))
			//this.SaveCache()
			// 这里演示一种直接保存数据库的用法,可以用于那些不经常修改的数据
			// 这种方式,省去了要处理crash后从缓存恢复数据的步骤
			gentity.SaveEntityChangedDataToDb(GetGuildDb(), routineEntity, cache.Get(), false)
		},
		AfterTimerExecuteFunc: func(routineEntity gentity.Entity, t time.Time) {
			gentity.SaveEntityChangedDataToDb(GetGuildDb(), routineEntity, cache.Get(), false)
		},
	})
}

func (this *Guild) processMessage(message *GuildMessage) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("recover:%v", err)
			logger.LogStack()
		}
	}()
	switch v := message.message.(type) {
	case *pb.GuildDataViewReq:
		logger.Debug("%v", v)
		this.OnGuildDataViewReq(message, v)
	case *pb.GuildJoinReq:
		logger.Debug("%v", v)
		this.OnGuildJoinReq(message, v)
	case *pb.GuildJoinAgreeReq:
		logger.Debug("%v", v)
		this.OnGuildJoinAgreeReq(message, v)
	default:
		logger.Debug("ignore %v", proto.MessageName(v))
	}
}

func (this *Guild) GetMember(playerId int64) *pb.GuildMemberData {
	return this.Members.Get(playerId)
}

// 路由玩家消息
// this server -> other server -> player
func (this *Guild) RoutePlayerPacket(guildMessage *GuildMessage, cmd PacketCommand, message proto.Message) {
	game.RoutePlayerPacket(guildMessage.fromPlayerId, cmd, message,
		game.NewRouteOptions().SetToServerId(guildMessage.fromServerId))
}

// 路由玩家消息,直接发给客户端
// this server -> other server -> client
func (this *Guild) RouteClientPacket(guildMessage *GuildMessage, cmd PacketCommand, message proto.Message) {
	game.RoutePlayerPacket(guildMessage.fromPlayerId, cmd, message,
		game.DirectSendClientRouteOptions().SetToServerId(guildMessage.fromServerId))
}

// 广播公会消息
// this server -> other server -> player
func (this *Guild) BroadcastPlayerPacket(cmd PacketCommand, message proto.Message) {
	for _, member := range this.Members.Data {
		game.RoutePlayerPacket(member.Id, cmd, message)
	}
}

// 广播公会消息,直接发给客户端
// this server -> other server -> client
func (this *Guild) BroadcastClientPacket(cmd PacketCommand, message proto.Message) {
	for _, member := range this.Members.Data {
		game.RoutePlayerPacket(member.Id, cmd, message, game.DirectSendClientRouteOptions())
	}
}

// 加入公会请求
func (this *Guild) OnGuildJoinReq(message *GuildMessage, req *pb.GuildJoinReq) {
	if this.GetMember(message.fromPlayerId) != nil {
		return
	}
	if this.JoinRequests.Get(message.fromPlayerId) != nil {
		return
	}
	this.JoinRequests.Add(&pb.GuildJoinRequest{
		PlayerId:     message.fromPlayerId,
		PlayerName:   message.fromPlayerName,
		TimestampSec: int32(util.GetCurrentTimeStamp()),
	})
	this.RouteClientPacket(message, PacketCommand(pb.CmdGuild_Cmd_GuildJoinRes), &pb.GuildJoinRes{
		Id: this.GetId(),
	})
	this.BroadcastClientPacket(PacketCommand(pb.CmdGuild_Cmd_GuildJoinReqTip), &pb.GuildJoinReqTip{
		PlayerId:   message.fromPlayerId,
		PlayerName: message.fromPlayerName,
	})
	logger.Debug("JoinRequests %v %v", this.GetId(), message.fromPlayerId)
}

// 公会管理者同意申请人加入公会
func (this *Guild) OnGuildJoinAgreeReq(message *GuildMessage, req *pb.GuildJoinAgreeReq) {
	member := this.GetMember(message.fromPlayerId)
	if member == nil {
		return
	}
	if member.Position < int32(pb.GuildPosition_Manager) {
		return
	}
	joinRequest := this.JoinRequests.Get(req.JoinPlayerId)
	if joinRequest == nil {
		return
	}
	if req.IsAgree {
		// TODO:检查该玩家是否已经有公会了
		this.Members.Add(&pb.GuildMemberData{
			Id:       joinRequest.PlayerId,
			Name:     joinRequest.PlayerName,
			Position: int32(pb.GuildPosition_Member),
		})
		this.BaseInfo.SetMemberCount(int32(len(this.Members.Data)))
	}
	this.JoinRequests.Remove(req.JoinPlayerId)
	// 返回操作结果
	this.RouteClientPacket(message, PacketCommand(pb.CmdGuild_Cmd_GuildJoinAgreeRes), &pb.GuildJoinAgreeRes{
		GuildId:         this.GetId(),
		ManagerPlayerId: member.Id,
		JoinPlayerId:    joinRequest.PlayerId,
		IsAgree:         req.IsAgree,
	})
	game.RoutePlayerPacket(joinRequest.PlayerId, PacketCommand(pb.CmdGuild_Cmd_GuildJoinAgreeRes), &pb.GuildJoinAgreeRes{
		GuildId:         this.GetId(),
		ManagerPlayerId: member.Id,
		JoinPlayerId:    joinRequest.PlayerId,
		IsAgree:         req.IsAgree,
	}, game.SaveDbRouteOptions())
}

// 查看公会数据
func (this *Guild) OnGuildDataViewReq(message *GuildMessage, req *pb.GuildDataViewReq) {
	if this.GetMember(message.fromPlayerId) == nil {
		return
	}
	this.RouteClientPacket(message, PacketCommand(pb.CmdGuild_Cmd_GuildDataViewRes), &pb.GuildDataViewRes{
		GuildData: &pb.GuildData{
			Id:           this.GetId(),
			BaseInfo:     this.BaseInfo.Data,
			Members:      this.Members.Data,
			JoinRequests: this.JoinRequests.Data,
		},
	})
}
