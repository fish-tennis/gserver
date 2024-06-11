package social

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/game"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

const (
	// 组件名
	ComponentNameJoinRequests = "JoinRequests"
)

// 利用go的init进行组件的自动注册
func init() {
	RegisterGuildComponentCtor(ComponentNameJoinRequests, 0, func(guild *Guild, guildData *pb.GuildLoadData) gentity.Component {
		component := &GuildJoinRequests{
			MapDataComponent: *gentity.NewMapDataComponent(guild, ComponentNameJoinRequests),
			Data:             make(map[int64]*pb.GuildJoinRequest),
		}
		gentity.LoadData(component, guildData.GetJoinRequests())
		return component
	})
}

// 公会加入请求
type GuildJoinRequests struct {
	gentity.MapDataComponent
	Data map[int64]*pb.GuildJoinRequest `db:"joinrequests"`
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
	this.Data[joinRequest.PlayerId] = joinRequest
	this.SetDirty(joinRequest.PlayerId, true)
	logger.Debug("Add request:%v", joinRequest)
}

func (this *GuildJoinRequests) Remove(playerId int64) {
	delete(this.Data, playerId)
	this.SetDirty(playerId, false)
	logger.Debug("Remove request:%v", playerId)
}

// 加入公会请求
func (this *GuildJoinRequests) HandleGuildJoinReq(guildMessage *GuildMessage, req *pb.GuildJoinReq) {
	errStr := ""
	g := this.GetGuild()
	logger.Debug("HandleGuildJoinReq %v %v", g.GetId(), guildMessage.fromPlayerId)
	defer g.RoutePlayerPacket(guildMessage, pb.CmdGuild_Cmd_GuildJoinRes, &pb.GuildJoinRes{
		Error: errStr,
		Id:    g.GetId(),
	})
	if g.GetMember(guildMessage.fromPlayerId) != nil {
		errStr = "already a member"
		return
	}
	if this.Get(guildMessage.fromPlayerId) != nil {
		errStr = "already have a join request"
		return
	}
	this.Add(&pb.GuildJoinRequest{
		PlayerId:     guildMessage.fromPlayerId,
		PlayerName:   guildMessage.fromPlayerName,
		TimestampSec: int32(util.GetCurrentTimeStamp()),
	})
	// 广播公会成员
	g.BroadcastClientPacket(pb.CmdGuild_Cmd_GuildJoinReqTip, &pb.GuildJoinReqTip{
		PlayerId:   guildMessage.fromPlayerId,
		PlayerName: guildMessage.fromPlayerName,
	})
}

// 公会管理员处理申请者的入会申请
func (this *GuildJoinRequests) HandleGuildJoinAgreeReq(guildMessage *GuildMessage, req *pb.GuildJoinAgreeReq) {
	g := this.GetGuild()
	logger.Debug("HandleGuildJoinAgreeReq %v %v", g.GetId(), guildMessage.fromPlayerId)
	errStr := ""
	// 返回操作结果
	defer g.RoutePlayerPacket(guildMessage, pb.CmdGuild_Cmd_GuildJoinAgreeRes, &pb.GuildJoinAgreeRes{
		Error:           errStr,
		GuildId:         g.GetId(),
		ManagerPlayerId: guildMessage.fromPlayerId,
		JoinPlayerId:    req.JoinPlayerId,
		IsAgree:         req.IsAgree,
	})
	member := g.GetMember(guildMessage.fromPlayerId)
	if member == nil {
		errStr = "not a member"
		return
	}
	if member.Position < int32(pb.GuildPosition_Manager) {
		errStr = "not a manager"
		return
	}
	joinRequest := g.GetJoinRequests().Get(req.JoinPlayerId)
	if joinRequest == nil {
		errStr = "no joinRequest"
		return
	}
	if g.GetMember(req.JoinPlayerId) != nil {
		errStr = "already joined"
		return
	}
	if req.IsAgree {
		// TODO:如果玩家之前已经提交了一个加入其他联盟的请求,玩家又自己创建联盟
		// 其他联盟的管理员又接受了该玩家的加入请求,如何防止该玩家同时存在于2个联盟?
		// 利用mongodb加一个类似原子锁的操作?
		g.GetMembers().Add(&pb.GuildMemberData{
			Id:       joinRequest.PlayerId,
			Name:     joinRequest.PlayerName,
			Position: int32(pb.GuildPosition_Member),
		})
		g.GetBaseInfo().SetMemberCount(int32(len(g.GetMembers().Data)))
		// 通知对方已经入会了
		game.RoutePlayerPacket(joinRequest.PlayerId, gnet.NewProtoPacketEx(pb.CmdGuild_Cmd_GuildJoinReqOpResult, &pb.GuildJoinReqOpResult{
			GuildId:         g.GetId(),
			ManagerPlayerId: guildMessage.fromPlayerId,
			JoinPlayerId:    joinRequest.PlayerId,
			IsAgree:         true,
		}), game.WithSaveDb())
	}
	g.GetJoinRequests().Remove(req.JoinPlayerId)
}
