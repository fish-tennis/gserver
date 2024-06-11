package social

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

const (
	// 组件名
	ComponentNameBaseInfo = "BaseInfo"
)

// 利用go的init进行组件的自动注册
func init() {
	RegisterGuildComponentCtor(ComponentNameBaseInfo, 0, func(guild *Guild, guildData *pb.GuildLoadData) gentity.Component {
		component := &GuildBaseInfo{
			DataComponent: *gentity.NewDataComponent(guild, ComponentNameBaseInfo),
			Data:          &pb.GuildInfo{},
		}
		gentity.LoadData(component, guildData.GetBaseInfo())
		return component
	})
}

var _ gentity.SaveableDirtyMark = (*GuildBaseInfo)(nil)

// 公会基础信息
type GuildBaseInfo struct {
	gentity.DataComponent
	Data *pb.GuildInfo `db:"baseinfo;plain"`
}

func (g *Guild) GetBaseInfo() *GuildBaseInfo {
	return g.GetComponentByName(ComponentNameBaseInfo).(*GuildBaseInfo)
}

func (this *GuildBaseInfo) GetGuild() *Guild {
	return this.GetEntity().(*Guild)
}

func (this *GuildBaseInfo) SetMemberCount(memberCount int32) {
	this.Data.MemberCount = memberCount
	this.SetDirty()
}

func (this *GuildBaseInfo) HandleGuildDataViewReq(guildMessage *GuildMessage, req *pb.GuildDataViewReq) {
	g := this.GetGuild()
	logger.Debug("HandleGuildDataViewReq %v %v", g.GetId(), guildMessage.fromPlayerId)
	if g.GetMember(guildMessage.fromPlayerId) == nil {
		logger.Debug("HandleGuildDataViewReq not a member %v %v", g.GetId(), guildMessage.fromPlayerId)
		return
	}
	g.RouteClientPacket(guildMessage, pb.CmdGuild_Cmd_GuildDataViewRes, &pb.GuildDataViewRes{
		GuildData: &pb.GuildData{
			Id:           g.GetId(),
			BaseInfo:     g.GetBaseInfo().Data,
			Members:      g.GetMembers().Data,
			JoinRequests: g.GetJoinRequests().Data,
		},
	})
}
