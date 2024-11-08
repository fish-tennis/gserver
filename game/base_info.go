package game

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

const (
	// 组件名
	ComponentNameBaseInfo = "BaseInfo"
)

// 利用go的init进行组件的自动注册
func init() {
	_playerComponentRegister.Register(ComponentNameBaseInfo, 0, func(player *Player, _ any) gentity.Component {
		return &BaseInfo{
			PlayerDataComponent: PlayerDataComponent{
				BasePlayerComponent: BasePlayerComponent{
					player: player,
					name:   ComponentNameBaseInfo,
				},
			},
			Data: &pb.BaseInfo{
				Level: 1,
				Exp:   0,
			},
		}
	})
}

// 玩家基础信息组件
type BaseInfo struct {
	PlayerDataComponent
	// plain表示明文存储,在保存到mongo时,不会进行proto序列化
	Data *pb.BaseInfo `db:"plain"`
}

func (this *Player) GetBaseInfo() *BaseInfo {
	return this.GetComponentByName(ComponentNameBaseInfo).(*BaseInfo)
}

// 玩家进游戏服成功,非客户端消息
// 这种格式写的函数可以自动注册非客户端的消息回调
func (this *BaseInfo) HandlePlayerEntryGameOk(msg *pb.PlayerEntryGameOk) {
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
	oldLevel := this.Data.Level
	this.Data.Exp += incExp
	for {
		if this.Data.Level < cfg.GetLevelCfgMgr().GetMaxLevel() {
			needExp := cfg.GetLevelCfgMgr().GetNeedExp(this.Data.Level + 1)
			if needExp > 0 && this.Data.Exp >= needExp {
				this.Data.Level++
				this.Data.Exp -= needExp
				continue
			}
		}
		break
	}
	logger.Debug("%v exp:%v lvl:%v", this.GetPlayerId(), this.Data.Exp, this.Data.Level)
	if oldLevel != this.Data.Level {
		this.GetPlayer().FireConditionEvent(&pb.EventPlayerProperty{
			PlayerId:      this.GetPlayerId(),
			PropertyName:  "Level",
			PropertyValue: this.Data.Level - oldLevel,
		})
	}
	// 修改了需要保存的数据后,必须设置标记
	this.SetDirty()
}

func (this *BaseInfo) TriggerPlayerExit(event *internal.EventPlayerExit) {
	this.Data.TotalOnlineSeconds += this.GetOnlineSecondsThisTime()
	this.Data.LastLogoutTimestamp = this.GetPlayer().GetTimerEntries().Now().Unix()
	this.SetDirty()
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
