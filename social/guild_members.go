package social

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

const (
	// 组件名
	ComponentNameMembers = "Members"
)

// 利用go的init进行组件的自动注册
func init() {
	RegisterGuildComponentCtor(ComponentNameMembers, 0, func(guild *Guild, guildData *pb.GuildLoadData) gentity.Component {
		component := &GuildMembers{
			MapDataComponent: *gentity.NewMapDataComponent(guild, ComponentNameMembers),
			Data:             make(map[int64]*pb.GuildMemberData),
		}
		gentity.LoadData(component, guildData.GetMembers())
		return component
	})
}

// 公会成员数据
type GuildMembers struct {
	gentity.MapDataComponent
	Data map[int64]*pb.GuildMemberData `db:"members;plain"`
}

func (this *Guild) GetMembers() *GuildMembers {
	return this.GetComponentByName(ComponentNameMembers).(*GuildMembers)
}

func (g *GuildMembers) Get(playerId int64) *pb.GuildMemberData {
	return g.Data[playerId]
}

func (g *GuildMembers) Add(member *pb.GuildMemberData) {
	g.Data[member.Id] = member
	g.SetDirty(member.Id, true)
	logger.Debug("Add member:%v", member)
}

func (g *GuildMembers) Remove(playerId int64) {
	delete(g.Data, playerId)
	g.SetDirty(playerId, false)
	logger.Debug("Remove member:%v", playerId)
}
