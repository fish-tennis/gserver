package social

import (
	"context"
	. "github.com/fish-tennis/gnet"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"google.golang.org/protobuf/proto"
	"sync"
)

var _ Entity = (*Guild)(nil)

// 公会
type Guild struct {
	BaseEntity
	messages chan *GuildMessage
	stopChan chan struct{}
	stopOnce sync.Once

	baseInfo     *GuildBaseInfo
	members      *GuildMembers
	joinRequests *GuildJoinRequests
}

type GuildMessage struct {
	fromPlayerId   int64
	fromServerId   int32
	fromPlayerName string
	cmd            PacketCommand
	message        proto.Message
}

func NewGuild(guildData *pb.GuildData) *Guild {
	guild := &Guild{
		messages: make(chan *GuildMessage, 32),
		stopChan: make(chan struct{}, 1),
	}
	guild.baseInfo = NewGuildBaseInfo(guild, guildData.BaseInfo)
	guild.AddComponent(guild.baseInfo, nil)
	guild.members = NewGuildMembers(guild, guildData.Members)
	guild.AddComponent(guild.members, nil)
	guild.joinRequests = NewGuildJoinRequests(guild, guildData.JoinRequests)
	guild.AddComponent(guild.joinRequests, nil)
	return guild
}

func (this *Guild) GetId() int64 {
	return this.baseInfo.Data.Id
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
				SaveEntityChangedDataToDb(GetGuildDb(), this, false)
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
			SaveEntityChangedDataToDb(GetGuildDb(), this, false)
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
	return this.members.Get(playerId)
}

// 路由玩家消息
func (this *Guild) RoutePlayerPacket(guildMessage *GuildMessage, cmd PacketCommand, message proto.Message) {
	RoutePlayerPacket(guildMessage.fromPlayerId, guildMessage.fromServerId, cmd, message)
}

// 加入公会请求
func (this *Guild) OnGuildJoinReq(message *GuildMessage, req *pb.GuildJoinReq) {
	if this.GetMember(message.fromPlayerId) != nil {
		return
	}
	if this.joinRequests.Get(message.fromPlayerId) != nil {
		return
	}
	this.joinRequests.Add(&pb.GuildJoinRequest{
		PlayerId:     message.fromPlayerId,
		PlayerName:   message.fromPlayerName,
		TimestampSec: int32(util.GetCurrentTimeStamp()),
	})
	this.RoutePlayerPacket(message, PacketCommand(pb.CmdGuild_Cmd_GuildJoinRes), &pb.GuildJoinRes{
		Id: this.GetId(),
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
	joinRequest := this.joinRequests.Get(req.JoinPlayerId)
	if joinRequest == nil {
		return
	}
	if req.IsAgree {
		// TODO:检查该玩家是否已经有公会了
		this.members.Add(&pb.GuildMemberData{
			Id:       joinRequest.PlayerId,
			Name:     joinRequest.PlayerName,
			Position: int32(pb.GuildPosition_Member),
		})
		this.baseInfo.SetMemberCount(int32(len(this.members.Data)))
	} else {
		// 略:给该玩家发一个提示信息
	}
	this.joinRequests.Remove(req.JoinPlayerId)
	this.RoutePlayerPacket(message, PacketCommand(pb.CmdGuild_Cmd_GuildJoinAgreeRes), &pb.GuildJoinAgreeRes{
		JoinPlayerId: joinRequest.PlayerId,
		IsAgree:      req.IsAgree,
	})
}

// 查看公会数据
func (this *Guild) OnGuildDataViewReq(message *GuildMessage, req *pb.GuildDataViewReq) {
	if this.GetMember(message.fromPlayerId) == nil {
		return
	}
	this.RoutePlayerPacket(message, PacketCommand(pb.CmdGuild_Cmd_GuildDataViewRes), &pb.GuildDataViewRes{
		GuildData: &pb.GuildData{
			Id:           this.GetId(),
			BaseInfo:     this.baseInfo.Data,
			Members:      this.members.Data,
			JoinRequests: this.joinRequests.Data,
		},
	})
}
