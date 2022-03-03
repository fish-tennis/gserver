package social

import (
	"fmt"
	"github.com/fish-tennis/gserver/pb"
	"strings"
	. "github.com/fish-tennis/gserver/internal"
)

var _ Saveable = (*GuildBaseInfo)(nil)
// 公会基础信息
type GuildBaseInfo struct {
	DataComponent
	guild *Guild
	data *pb.GuildInfo
}

func (g *GuildBaseInfo) DbData() (dbData interface{}, protoMarshal bool) {
	return g.data,false
}

func (g *GuildBaseInfo) CacheData() interface{} {
	return g.data
}

func (g *GuildBaseInfo) GetCacheKey() string {
	return fmt.Sprintf("g.%v.{%v}", strings.ToLower(g.GetName()), g.guild.GetId())
}

var _ Saveable = (*GuildMembers)(nil)
// 公会成员数据
type GuildMembers struct {
	MapDataComponent
	guild *Guild
	data map[int64]*pb.GuildMemberData
}

func (g *GuildMembers) DbData() (dbData interface{}, protoMarshal bool) {
	return g.data,false
}

func (g *GuildMembers) CacheData() interface{} {
	return g.data
}

func (g *GuildMembers) GetCacheKey() string {
	return fmt.Sprintf("g.%v.{%v}", strings.ToLower(g.GetName()), g.guild.GetId())
}


var _ Saveable = (*GuildJoinRequests)(nil)
// 公会加入请求
type GuildJoinRequests struct {
	MapDataComponent
	guild *Guild
	data map[int64]*pb.GuildJoinRequest
}

func (g *GuildJoinRequests) DbData() (dbData interface{}, protoMarshal bool) {
	return g.data,false
}

func (g *GuildJoinRequests) CacheData() interface{} {
	return g.data
}

func (g *GuildJoinRequests) GetCacheKey() string {
	return fmt.Sprintf("g.%v.{%v}", strings.ToLower(g.GetName()), g.guild.GetId())
}
