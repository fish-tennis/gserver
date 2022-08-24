package social

import (
	"context"
	"github.com/fish-tennis/gentity/util"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/gameplayer"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gentity"
	"google.golang.org/protobuf/proto"
	"sync"
)

var _ gentity.Entity = (*Guild)(nil)

// 公会
type Guild struct {
	gentity.BaseEntity
	messages chan *GuildMessage
	stopChan chan struct{}
	stopOnce sync.Once

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
		messages: make(chan *GuildMessage, 32),
		stopChan: make(chan struct{}, 1),
	}
	guild.BaseInfo = NewGuildBaseInfo(guild, guildData.BaseInfo)
	guild.AddComponent(guild.BaseInfo, nil)
	guild.Members = NewGuildMembers(guild, guildData.Members)
	guild.AddComponent(guild.Members, nil)
	guild.JoinRequests = NewGuildJoinRequests(guild)
	guild.AddComponent(guild.JoinRequests, guildData.JoinRequests)
	return guild
}

func (this *Guild) GetId() int64 {
	return this.BaseInfo.Data.Id
}

func (this *Guild) PushMessage(guildMessage *GuildMessage) {
	this.messages <- guildMessage
}

// 开启消息处理协程
func (this *Guild) RunProcessRoutine() bool {
	logger.Debug("RunProcessRoutine %v", this.GetId())
	// redis实现的分布式锁,保证同一个公会的逻辑处理协程只会在一个服务器上
	if !guildServerLock(this.GetId()) {
		return false
	}
	GetServer().GetWaitGroup().Add(1)
	go func(ctx context.Context) {
		defer func() {
			// 协程结束的时候,分布式锁UnLock
			guildServerUnlock(this.GetId())
			//SaveEntityToDb(GetGuildDb(), this, true)
			GetServer().GetWaitGroup().Done()
			if err := recover(); err != nil {
				logger.Error("recover:%v", err)
				logger.LogStack()
			}
			logger.Debug("EndProcessRoutine %v", this.GetId())
		}()

		for {
			select {
			case <-ctx.Done():
				logger.Info("exitNotify %v", this.GetId())
				goto END
			case <-this.stopChan:
				logger.Debug("stop %v", this.GetId())
				goto END
			case message := <-this.messages:
				// nil消息 表示这是需要处理的最后一条消息
				if message == nil {
					return
				}
				this.processMessage(message)
				//this.SaveCache()
				// 这里演示一种直接保存数据库的用法,可以用于那些不经常修改的数据
				// 这种方式,省去了要处理crash后从缓存恢复数据的步骤
				gentity.SaveEntityChangedDataToDb(GetGuildDb(), this, cache.Get(), false)
			}
		}

		// 有可能还有未处理的消息
	END:
		messageLen := len(this.messages)
		for i := 0; i < messageLen; i++ {
			message := <-this.messages
			// nil消息 表示这是需要处理的最后一条消息
			if message == nil {
				return
			}
			this.processMessage(message)
			gentity.SaveEntityChangedDataToDb(GetGuildDb(), this, cache.Get(), false)
		}
	}(GetServer().GetContext())
	return true
}

func (this *Guild) Stop() {
	this.stopOnce.Do(func() {
		this.stopChan <- struct{}{}
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
	gameplayer.RoutePlayerPacket(guildMessage.fromPlayerId, cmd, message,
		gameplayer.NewRouteOptions().SetToServerId(guildMessage.fromServerId))
}

// 路由玩家消息,直接发给客户端
// this server -> other server -> client
func (this *Guild) RouteClientPacket(guildMessage *GuildMessage, cmd PacketCommand, message proto.Message) {
	gameplayer.RoutePlayerPacket(guildMessage.fromPlayerId, cmd, message,
		gameplayer.DirectSendClientRouteOptions().SetToServerId(guildMessage.fromServerId))
}

// 广播公会消息
// this server -> other server -> player
func (this *Guild) BroadcastPlayerPacket(cmd PacketCommand, message proto.Message) {
	for _, member := range this.Members.Data {
		gameplayer.RoutePlayerPacket(member.Id, cmd, message)
	}
}

// 广播公会消息,直接发给客户端
// this server -> other server -> client
func (this *Guild) BroadcastClientPacket(cmd PacketCommand, message proto.Message) {
	for _, member := range this.Members.Data {
		gameplayer.RoutePlayerPacket(member.Id, cmd, message, gameplayer.DirectSendClientRouteOptions())
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
	gameplayer.RoutePlayerPacket(joinRequest.PlayerId, PacketCommand(pb.CmdGuild_Cmd_GuildJoinAgreeRes), &pb.GuildJoinAgreeRes{
		GuildId:         this.GetId(),
		ManagerPlayerId: member.Id,
		JoinPlayerId:    joinRequest.PlayerId,
		IsAgree:         req.IsAgree,
	}, gameplayer.SaveDbRouteOptions())
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
