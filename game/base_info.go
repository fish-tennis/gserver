package game

import (
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

// 玩家基础信息组件
type BaseInfo struct {
	PlayerDataComponent
	// plain表示明文存储,在保存到mongo时,不会进行proto序列化
	Data *pb.BaseInfo `db:"BaseInfo;plain"`
}

func NewBaseInfo(player *Player, data *pb.BaseInfo) *BaseInfo {
	component := &BaseInfo{
		PlayerDataComponent: PlayerDataComponent{
			BasePlayerComponent: BasePlayerComponent{
				player: player,
				name:   "BaseInfo",
			},
		},
		Data: &pb.BaseInfo{
			Level: 1,
			Exp:   0,
		},
	}
	if data != nil {
		component.Data = data
	}
	return component
}

// 玩家进游戏服成功,非客户端消息
// 这种格式写的函数可以自动注册非客户端的消息回调
func (this *BaseInfo) HandlePlayerEntryGameOk(_ gnet.PacketCommand, msg *pb.PlayerEntryGameOk) {
	logger.Debug("HandlePlayerEntryGameOk:%v", msg)
	now := this.GetPlayer().GetTimerEntries().Now().Unix()
	var offlineSeconds int32
	if this.Data.LastLogoutTimestamp > 0 && now > this.Data.LastLogoutTimestamp {
		offlineSeconds = int32(now - this.Data.LastLogoutTimestamp)
	}
	this.Data.LastLoginTimestamp = now
	this.SetDirty()
	// 分发事件:玩家进游戏服
	this.GetPlayer().FireEvent(&internal.EventPlayerEntryGame{
		IsReconnect:    msg.IsReconnect,
		OfflineSeconds: offlineSeconds,
	})
}

func (this *BaseInfo) IncExp(incExp int32) {
	this.Data.Exp += incExp
	for {
		if this.Data.Level < cfg.GetLevelCfgMgr().GetMaxLevel() {
			needExp := cfg.GetLevelCfgMgr().GetNeedExp(this.Data.Level + 1)
			if needExp > 0 && this.Data.Exp >= needExp {
				this.Data.Level++
				this.Data.Exp -= needExp
				this.GetPlayer().FireConditionEvent(&pb.EventPlayerLevelup{
					PlayerId: this.GetPlayerId(),
					Level:    this.Data.Level,
				})
				continue
			}
		}
		break
	}
	logger.Debug("%v exp:%v lvl:%v", this.GetPlayerId(), this.Data.Exp, this.Data.Level)
	// 修改了需要保存的数据后,必须设置标记
	this.SetDirty()
}

func (this *BaseInfo) OnEvent(event interface{}) {
	switch event.(type) {
	case *internal.EventPlayerExit:
		this.Data.TotalOnlineSeconds += this.GetOnlineSecondsThisTime()
		this.Data.LastLogoutTimestamp = this.GetPlayer().GetTimerEntries().Now().Unix()
		this.SetDirty()
	}
}

// 本次登录在线时长
func (this *BaseInfo) GetOnlineSecondsThisTime() int32 {
	now := this.GetPlayer().GetTimerEntries().Now().Unix()
	var onlineSeconds int32
	if this.Data.LastLoginTimestamp > 0 && now > this.Data.LastLoginTimestamp {
		onlineSeconds = int32(now - this.Data.LastLoginTimestamp)
	}
	return onlineSeconds
}

// 总在线时长
func (this *BaseInfo) GetTotalOnlineSeconds() int32 {
	return this.Data.TotalOnlineSeconds + this.GetOnlineSecondsThisTime()
}
