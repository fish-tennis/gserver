package social

import (
	"errors"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gserver/game"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
)

const (
	// 组件名
	ComponentNameJoinRequests = "JoinRequests"
)

// 利用go的init进行组件的自动注册
func init() {
	_guildComponentRegister.Register(ComponentNameJoinRequests, 0, func(guild *Guild, _ any) gentity.Component {
		return &GuildJoinRequests{
			MapDataComponent: gentity.NewMapDataComponent[int64, *pb.GuildJoinRequest](guild, ComponentNameJoinRequests),
		}
	})
}

// 公会加入请求
type GuildJoinRequests struct {
	*gentity.MapDataComponent[int64, *pb.GuildJoinRequest] `db:""`
}

func (g *Guild) GetJoinRequests() *GuildJoinRequests {
	return g.GetComponentByName(ComponentNameJoinRequests).(*GuildJoinRequests)
}

func (this *GuildJoinRequests) GetGuild() *Guild {
	return this.GetEntity().(*Guild)
}

func (this *GuildJoinRequests) Get(playerId int64) *pb.GuildJoinRequest {
	return this.Data[playerId]
}

func (this *GuildJoinRequests) Add(joinRequest *pb.GuildJoinRequest) {
	this.Set(joinRequest.PlayerId, joinRequest)
	logger.Debug("Add request:%v", joinRequest)
}

func (this *GuildJoinRequests) Remove(playerId int64) {
	this.Delete(playerId)
	logger.Debug("Remove request:%v", playerId)
}

// 加入公会请求
func (this *GuildJoinRequests) HandleGuildJoinReq(guildMessage *GuildMessage, req *pb.GuildJoinReq) (*pb.GuildJoinRes, error) {
	g := this.GetGuild()
	slog.Debug("HandleGuildJoinReq", "gid", g.GetId(), "pid", guildMessage.fromPlayerId)
	if g.GetMember(guildMessage.fromPlayerId) != nil {
		return nil, errors.New("already a member")
	}
	if this.Get(guildMessage.fromPlayerId) != nil {
		return nil, errors.New("already have a join request")
	}
	this.Add(&pb.GuildJoinRequest{
		PlayerId:     guildMessage.fromPlayerId,
		PlayerName:   guildMessage.fromPlayerName,
		TimestampSec: int32(util.GetCurrentTimeStamp()),
	})
	// 广播公会成员
	g.BroadcastClientPacket(&pb.GuildJoinReqTip{
		PlayerId:   guildMessage.fromPlayerId,
		PlayerName: guildMessage.fromPlayerName,
	})
	return &pb.GuildJoinRes{
		Id: g.GetId(),
	}, nil
}

// 公会管理员处理申请者的入会申请
func (this *GuildJoinRequests) HandleGuildJoinAgreeReq(guildMessage *GuildMessage, req *pb.GuildJoinAgreeReq) (*pb.GuildJoinAgreeRes, error) {
	g := this.GetGuild()
	logger.Debug("HandleGuildJoinAgreeReq %v %v", g.GetId(), guildMessage.fromPlayerId)
	member := g.GetMember(guildMessage.fromPlayerId)
	if member == nil {
		return nil, errors.New("not a member")
	}
	if member.Position < int32(pb.GuildPosition_Manager) {
		return nil, errors.New("not a manager")
	}
	joinRequest := g.GetJoinRequests().Get(req.JoinPlayerId)
	if joinRequest == nil {
		return nil, errors.New("no joinRequest")
	}
	if g.GetMember(req.JoinPlayerId) != nil {
		return nil, errors.New("already joined")
	}
	if req.IsAgree {
		g.GetMembers().Add(&pb.GuildMemberData{
			Id:       joinRequest.PlayerId,
			Name:     joinRequest.PlayerName,
			Position: int32(pb.GuildPosition_Member),
		})
		// 利用mongodb的原子操作,来防止该玩家同时加入多个公会
		if !game.AtomicSetGuildId(joinRequest.PlayerId, g.GetId(), 0) {
			g.GetMembers().Remove(joinRequest.PlayerId)
			return nil, errors.New("ConcurrentError")
		}
		// 通知对方已经入会了
		// 这里使用了WithSaveDb选项,如果玩家此时不在线,等他下次上线时,会收到该消息
		game.RoutePlayerPacket(joinRequest.PlayerId, network.NewPacket(&pb.GuildJoinReqOpResult{
			GuildId:         g.GetId(),
			ManagerPlayerId: guildMessage.fromPlayerId,
			JoinPlayerId:    joinRequest.PlayerId,
			IsAgree:         true,
		}), game.WithSaveDb())
	}
	g.GetJoinRequests().Remove(req.JoinPlayerId)
	return &pb.GuildJoinAgreeRes{
		Error:           "",
		GuildId:         g.GetId(),
		ManagerPlayerId: guildMessage.fromPlayerId,
		JoinPlayerId:    req.JoinPlayerId,
		IsAgree:         req.IsAgree,
	}, nil
}
