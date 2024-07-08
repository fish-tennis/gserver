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
			MapDataComponent: *gentity.NewMapDataComponent[int64, *pb.GuildMemberData](guild, ComponentNameMembers),
		}
		gentity.LoadData(component, guildData.GetMembers())
		return component
	})
}

// 公会成员数据
type GuildMembers struct {
	gentity.MapDataComponent[int64, *pb.GuildMemberData] `db:"Members;plain"`
}

func (g *Guild) GetMembers() *GuildMembers {
	return g.GetComponentByName(ComponentNameMembers).(*GuildMembers)
}

func (this *GuildMembers) GetGuild() *Guild {
	return this.GetEntity().(*Guild)
}

func (this *GuildMembers) Get(playerId int64) *pb.GuildMemberData {
	return this.Data[playerId]
}

func (this *GuildMembers) Add(member *pb.GuildMemberData) {
	this.Set(member.Id, member)
	this.GetGuild().GetBaseInfo().SetMemberCount(int32(len(this.Data)))
	logger.Debug("Add member:%v", member)
}

func (this *GuildMembers) Remove(playerId int64) {
	this.Delete(playerId)
	this.GetGuild().GetBaseInfo().SetMemberCount(int32(len(this.Data)))
	logger.Debug("Remove member:%v", playerId)
}
