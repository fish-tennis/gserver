package social

import (
	"errors"
	"log/slog"

	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/pb"
)

const (
	// 组件名
	ComponentNameBaseInfo = "BaseInfo"
)

// 利用go的init进行组件的自动注册
func init() {
	_guildComponentRegister.Register(ComponentNameBaseInfo, 0, func(guild *Guild, _ any) gentity.Component {
		return &GuildBaseInfo{
			DataComponent: *gentity.NewDataComponent(guild, ComponentNameBaseInfo),
			Data:          &pb.GuildInfo{},
		}
	})
}

var _ gentity.SaveableDirtyMark = (*GuildBaseInfo)(nil)

// 公会基础信息
type GuildBaseInfo struct {
	gentity.DataComponent
	Data *pb.GuildInfo `db:"plain"`
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

func (this *GuildBaseInfo) HandleGuildDataViewReq(guildMessage *GuildMessage, req *pb.GuildDataViewReq) (*pb.GuildDataViewRes, error) {
	g := this.GetGuild()
	slog.Debug("HandleGuildDataViewReq", "gid", g.GetId(), "pid", guildMessage.fromPlayerId)
	if g.GetMember(guildMessage.fromPlayerId) == nil {
		slog.Debug("HandleGuildDataViewReq not a member", "gid", g.GetId(), "pid", guildMessage.fromPlayerId)
		return nil, errors.New("not a member")
	}
	return &pb.GuildDataViewRes{
		GuildData: &pb.GuildData{
			Id:           g.GetId(),
			BaseInfo:     g.GetBaseInfo().Data,
			Members:      g.GetMembers().Data,
			JoinRequests: g.GetJoinRequests().Data,
		},
	}, nil
}
