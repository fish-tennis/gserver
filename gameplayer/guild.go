package gameplayer

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
)

var _ internal.Saveable = (*Guild)(nil)

// 玩家的公会模块
type Guild struct {
	PlayerDataComponent
	Data *pb.PlayerGuildData `db:"guild"`
}

func NewGuild(player *Player) *Guild {
	component := &Guild{
		PlayerDataComponent: *NewPlayerDataComponent(player,"Guild"),
		Data: &pb.PlayerGuildData{
			GuildId: 0,
		},
	}
	return component
}

func (this *Guild) DbData() (dbData interface{}, protoMarshal bool) {
	return this.Data,true
}

func (this *Guild) CacheData() interface{} {
	return this.Data
}

func (this *Guild) GetGuildData() *pb.PlayerGuildData {
	return this.Data
}

func (this *Guild) SetGuildId(guildId int64) {
	this.Data.GuildId = guildId
	this.SetDirty()
}

//// 事件接口
//func (this *Guild) OnEvent(event interface{}) {
//	switch v := event.(type) {
//	case *internal.EventPlayerEntryGame:
//		this.OnPlayerEntryGame(v)
//	}
//}