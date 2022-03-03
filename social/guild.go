package social

import (
	"fmt"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/gameplayer"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"google.golang.org/protobuf/proto"
)

var _ Saveable = (*Guild)(nil)

// 公会
type Guild struct {
	messages chan *GuildMessage
	BaseDirtyMark
	data *pb.GuildData
}

type GuildMessage struct {
	fromPlayerId  int64
	fromServerId  int32
	cmd PacketCommand
	message proto.Message
}

func NewGuild(id int64, name string) *Guild {
	return &Guild{
		messages: make(chan *GuildMessage, 32),
		data: &pb.GuildData{
			Id: id,
			Name: name,
			Members: make(map[int64]*pb.GuildMemberData),
			JoinRequests: make(map[int64]*pb.GuildJoinRequest),
		},
	}
}

func (this *Guild) DbData() (dbData interface{}, protoMarshal bool) {
	return this.data,false
}

func (this *Guild) CacheData() interface{} {
	return this.data
}

func (this *Guild) GetCacheKey() string {
	return fmt.Sprintf("guild:%v", this.data.Id)
}

func (this *Guild) PushMessage(guildMessage *GuildMessage) {
	this.messages <- guildMessage
}

// 消息处理协程
func (this *Guild) StartProcessRoutine() {
	logger.Debug("StartProcessRoutine %v", this.data.Id)
	ctx := GetServer().GetContext()
	GetServer().GetWaitGroup().Add(1)
	go func() {
		defer func() {
			GetServer().GetWaitGroup().Done()
			if err := recover(); err != nil {
				logger.LogStack()
			}
			logger.Debug("EndProcessRoutine %v", this.data.Id)
		}()

		for {
			select {
			case <-ctx.Done():
				logger.Info("exitNotify")
				return
			case message := <- this.messages:
				if message == nil {
					return
				}
				this.processMessage(message)
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
	return this.data.Members[playerId]
}

// 加入公会请求
func (this *Guild) OnGuildJoinReq(message *GuildMessage, req *pb.GuildJoinReq) {
	if this.GetMember(message.fromPlayerId) != nil {
		return
	}
	if _,ok := this.data.JoinRequests[message.fromPlayerId]; ok {
		return
	}
	this.data.JoinRequests[message.fromPlayerId] = &pb.GuildJoinRequest{
		PlayerId: message.fromPlayerId,
		PlayerName: "",
		TimestampSec: int32(util.GetCurrentTimeStamp()),
	}
	logger.Debug("JoinRequests %v %v", this.data.Id, message.fromPlayerId)
}

// 同意加入公会
func (this *Guild) OnGuildJoinAgreeReq(player *gameplayer.Player, req *pb.GuildJoinAgreeReq) {

}

// 查看公会数据
func (this *Guild) OnRequestGuildDataReq(player *gameplayer.Player, req *pb.RequestGuildDataReq) {

}
