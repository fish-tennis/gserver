package game

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

const (
	// 组件名
	ComponentNameGuild = "Guild"
)

// 利用go的init进行组件的自动注册
func init() {
	RegisterPlayerComponentCtor(ComponentNameGuild, 100, func(player *Player, playerData *pb.PlayerData) gentity.Component {
		component := &Guild{
			PlayerDataComponent: *NewPlayerDataComponent(player, ComponentNameGuild),
			Data: &pb.PlayerGuildData{
				GuildId: 0,
			},
		}
		gentity.LoadData(component, playerData.GetGuild())
		return component
	})
}

// 玩家的公会模块
type Guild struct {
	PlayerDataComponent
	Data *pb.PlayerGuildData `db:"Guild"`
}

func (this *Player) GetGuild() *Guild {
	return this.GetComponentByName(ComponentNameGuild).(*Guild)
}

func (this *Guild) GetGuildData() *pb.PlayerGuildData {
	return this.Data
}

func (this *Guild) SetGuildId(guildId int64) {
	this.Data.GuildId = guildId
	this.SetDirty()
	logger.Debug("%v SetGuildId %v", this.GetPlayerId(), guildId)
}

// 玩家进游戏服成功,非客户端消息
// 这种格式写的函数可以自动注册非客户端的消息回调
func (this *Guild) HandleGuildJoinAgreeRes(cmd gnet.PacketCommand, msg *pb.GuildJoinAgreeRes) {
	logger.Debug("HandleGuildJoinAgreeRes:%v", msg)
	if msg.IsAgree {
		this.SetGuildId(msg.GuildId)
	}
	this.GetPlayer().Send(cmd, msg)
}
