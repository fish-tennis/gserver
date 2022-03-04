package social

import (
	"fmt"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"strings"
	. "github.com/fish-tennis/gserver/internal"
)

var _ SaveableDirtyMark = (*GuildBaseInfo)(nil)
// 公会基础信息
type GuildBaseInfo struct {
	DataComponent
	data *pb.GuildInfo
}

func NewGuildBaseInfo(entity Entity, data *pb.GuildInfo) *GuildBaseInfo {
	c := &GuildBaseInfo{
		DataComponent:*NewDataComponent(entity,"BaseInfo"),
		data: data,
	}
	if c.data == nil {
		c.data = new(pb.GuildInfo)
	}
	return c
}

func (g *GuildBaseInfo) DbData() (dbData interface{}, protoMarshal bool) {
	return g.data,false
}

func (g *GuildBaseInfo) CacheData() interface{} {
	return g.data
}

func (g *GuildBaseInfo) GetCacheKey() string {
	return fmt.Sprintf("g.%v.{%v}", strings.ToLower(g.GetName()), g.GetEntity().GetId())
}

func (g *GuildBaseInfo) SetMemberCount(memberCount int32) {
	g.data.MemberCount = memberCount
	g.SetDirty()
}

var _ SaveableMapDirtyMark = (*GuildMembers)(nil)
// 公会成员数据
type GuildMembers struct {
	MapDataComponent
	data map[int64]*pb.GuildMemberData
}

func NewGuildMembers(entity Entity, data map[int64]*pb.GuildMemberData) *GuildMembers {
	c := &GuildMembers{
		MapDataComponent:*NewMapDataComponent(entity,"Members"),
		data: data,
	}
	if c.data == nil {
		c.data = make(map[int64]*pb.GuildMemberData)
	}
	return c
}

func (g *GuildMembers) DbData() (dbData interface{}, protoMarshal bool) {
	return g.data,false
}

func (g *GuildMembers) CacheData() interface{} {
	return g.data
}

func (g *GuildMembers) GetCacheKey() string {
	return fmt.Sprintf("g.%v.{%v}", strings.ToLower(g.GetName()), g.GetEntity().GetId())
}

func (g *GuildMembers) GetMapValue(key string) (value interface{}, exists bool) {
	value,exists = g.data[util.Atoi64(key)]
	return
}

func (g *GuildMembers) Get(playerId int64) *pb.GuildMemberData {
	return g.data[playerId]
}

func (g *GuildMembers) Add(member *pb.GuildMemberData) {
	g.data[member.Id] = member
	g.SetDirty(member.Id, true)
	logger.Debug("Add member:%v", member)
}

func (g *GuildMembers) Remove(playerId int64) {
	delete(g.data, playerId)
	g.SetDirty(playerId, false)
	logger.Debug("Remove member:%v", playerId)
}


var _ SaveableMapDirtyMark = (*GuildJoinRequests)(nil)
// 公会加入请求
type GuildJoinRequests struct {
	MapDataComponent
	data map[int64]*pb.GuildJoinRequest
}

func NewGuildJoinRequests(entity Entity, data map[int64]*pb.GuildJoinRequest) *GuildJoinRequests {
	c := &GuildJoinRequests{
		MapDataComponent:*NewMapDataComponent(entity,"JoinRequests"),
		data: data,
	}
	if c.data == nil {
		c.data = make(map[int64]*pb.GuildJoinRequest)
	}
	return c
}

func (g *GuildJoinRequests) DbData() (dbData interface{}, protoMarshal bool) {
	return g.data,false
}

func (g *GuildJoinRequests) CacheData() interface{} {
	return g.data
}

func (g *GuildJoinRequests) GetCacheKey() string {
	return fmt.Sprintf("g.%v.{%v}", strings.ToLower(g.GetName()), g.GetEntity().GetId())
}

func (g *GuildJoinRequests) GetMapValue(key string) (value interface{}, exists bool) {
	value,exists = g.data[util.Atoi64(key)]
	return
}

func (g *GuildJoinRequests) Get(playerId int64) *pb.GuildJoinRequest {
	return g.data[playerId]
}

func (g *GuildJoinRequests) Add(joinRequest *pb.GuildJoinRequest) {
	g.data[joinRequest.PlayerId] = joinRequest
	g.SetDirty(joinRequest.PlayerId, true)
}

func (g *GuildJoinRequests) Remove(playerId int64) {
	delete(g.data, playerId)
	g.SetDirty(playerId, false)
}