package social

import (
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

var _ SaveableDirtyMark = (*GuildBaseInfo)(nil)
// 公会基础信息
type GuildBaseInfo struct {
	DataComponent
	Data *pb.GuildInfo `db:"baseinfo;plain"`
}

func NewGuildBaseInfo(entity Entity, data *pb.GuildInfo) *GuildBaseInfo {
	c := &GuildBaseInfo{
		DataComponent: *NewDataComponent(entity,"BaseInfo"),
		Data:          data,
	}
	if c.Data == nil {
		c.Data = new(pb.GuildInfo)
	}
	return c
}

func (g *GuildBaseInfo) SetMemberCount(memberCount int32) {
	g.Data.MemberCount = memberCount
	g.SetDirty()
}

// 公会成员数据
type GuildMembers struct {
	MapDataComponent
	Data map[int64]*pb.GuildMemberData `db:"members"`
}

func NewGuildMembers(entity Entity, data map[int64]*pb.GuildMemberData) *GuildMembers {
	c := &GuildMembers{
		MapDataComponent: *NewMapDataComponent(entity,"Members"),
		Data:             data,
	}
	if c.Data == nil {
		c.Data = make(map[int64]*pb.GuildMemberData)
	}
	return c
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


// 公会加入请求
type GuildJoinRequests struct {
	MapDataComponent
	Data map[int64]*pb.GuildJoinRequest `db:"joinrequests"`
}

func NewGuildJoinRequests(entity Entity, data map[int64]*pb.GuildJoinRequest) *GuildJoinRequests {
	c := &GuildJoinRequests{
		MapDataComponent: *NewMapDataComponent(entity,"JoinRequests"),
		Data:             data,
	}
	if c.Data == nil {
		c.Data = make(map[int64]*pb.GuildJoinRequest)
	}
	return c
}

func (g *GuildJoinRequests) Get(playerId int64) *pb.GuildJoinRequest {
	return g.Data[playerId]
}

func (g *GuildJoinRequests) Add(joinRequest *pb.GuildJoinRequest) {
	g.Data[joinRequest.PlayerId] = joinRequest
	g.SetDirty(joinRequest.PlayerId, true)
}

func (g *GuildJoinRequests) Remove(playerId int64) {
	delete(g.Data, playerId)
	g.SetDirty(playerId, false)
}