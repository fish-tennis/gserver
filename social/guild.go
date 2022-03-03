package social

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/gameplayer"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"google.golang.org/protobuf/proto"
)

var _ Entity = (*Guild)(nil)

// 公会
type Guild struct {
	BaseEntity
	BaseDirtyMark
	messages chan *GuildMessage
	baseInfo *GuildBaseInfo
	members *GuildMembers
	joinRequests *GuildJoinRequests
}

type GuildMessage struct {
	fromPlayerId  int64
	fromServerId  int32
	cmd PacketCommand
	message proto.Message
}

func NewGuild(guildData *pb.GuildData) *Guild {
	guild := &Guild{
		messages: make(chan *GuildMessage, 32),
	}
	guild.baseInfo = &GuildBaseInfo{
		guild: guild,
		data: guildData.BaseInfo,
	}
	guild.AddComponent(guild.baseInfo, guildData.BaseInfo)

	guild.members = &GuildMembers{
		guild: guild,
		data: make(map[int64]*pb.GuildMemberData),
	}
	guild.AddComponent(guild.members, guildData.Members)
	guild.AddComponent(&GuildJoinRequests{
		guild: guild,
		data: make(map[int64]*pb.GuildJoinRequest),
	}, guildData.Members)
	return guild
}

func (this *Guild) GetId() int64 {
	return this.baseInfo.data.Id
}

func (this *Guild) PushMessage(guildMessage *GuildMessage) {
	this.messages <- guildMessage
}

// 消息处理协程
func (this *Guild) StartProcessRoutine() {
	logger.Debug("StartProcessRoutine %v", this.GetId())
	ctx := GetServer().GetContext()
	GetServer().GetWaitGroup().Add(1)
	go func() {
		defer func() {
			SaveEntityToDb(GetGuildDb(), this, true)
			GetServer().GetWaitGroup().Done()
			if err := recover(); err != nil {
				logger.LogStack()
			}
			logger.Debug("EndProcessRoutine %v", this.GetId())
		}()

		for {
			select {
			case <-ctx.Done():
				logger.Info("exitNotify")
				return
				// TODO:也可以加个定时保存db的功能
			case message := <- this.messages:
				if message == nil {
					return
				}
				this.processMessage(message)
				this.SaveCache()
			}
		}
	}()
}

func (this *Guild) processMessage(message *GuildMessage) {
	switch v := message.message.(type) {
	case *pb.GuildJoinReq:
		logger.Debug("%v", v)
		this.OnGuildJoinReq(message, v)
	default:
		logger.Debug("ignore %v", proto.MessageName(v))
	}
}

func (this *Guild) GetMember(playerId int64) *pb.GuildMemberData {
	return this.members.data[playerId]
}

func (this *Guild) RoutePlayerPacket(guildMessage *GuildMessage, cmd PacketCommand, message proto.Message) {
	RoutePlayerPacket(guildMessage.fromPlayerId, guildMessage.fromServerId, cmd, message)
}

// 加入公会请求
func (this *Guild) OnGuildJoinReq(message *GuildMessage, req *pb.GuildJoinReq) {
	if this.GetMember(message.fromPlayerId) != nil {
		return
	}
	if _,ok := this.joinRequests.data[message.fromPlayerId]; ok {
		return
	}
	this.joinRequests.data[message.fromPlayerId] = &pb.GuildJoinRequest{
		PlayerId: message.fromPlayerId,
		PlayerName: "",
		TimestampSec: int32(util.GetCurrentTimeStamp()),
	}
	this.RoutePlayerPacket(message, PacketCommand(pb.CmdGuild_Cmd_GuildJoinRes), &pb.GuildJoinRes{
		Id: this.GetId(),
	})
	logger.Debug("JoinRequests %v %v", this.GetId(), message.fromPlayerId)
}

// 同意加入公会
func (this *Guild) OnGuildJoinAgreeReq(player *gameplayer.Player, req *pb.GuildJoinAgreeReq) {

}

// 查看公会数据
func (this *Guild) OnRequestGuildDataReq(player *gameplayer.Player, req *pb.RequestGuildDataReq) {

}
