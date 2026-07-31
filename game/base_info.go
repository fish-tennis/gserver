package game

import (
	"fmt"
	"time"

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

func (p *Player) GetBaseInfo() *BaseInfo {
	return p.GetComponentByName(ComponentNameBaseInfo).(*BaseInfo)
}

func (b *BaseInfo) SyncDataToClient() {
	b.GetPlayer().Send(&pb.BaseInfoSync{
		Data: b.Data,
	})
}

func (b *BaseInfo) IncExp(incExp int32) {
	oldLevel := b.Data.Level
	// 防溢出:incExp 为负数时不允许低于 0
	newExp := int64(b.Data.Exp) + int64(incExp)
	if newExp < 0 {
		newExp = 0
	}
	b.Data.Exp = int32(newExp)
	for {
		if b.Data.Level < cfg.MaxLevel {
			needExp := cfg.GetNeedExp(b.Data.Level + 1)
			if needExp > 0 && b.Data.Exp >= needExp {
				b.Data.Level++
				b.Data.Exp -= needExp
				continue
			}
		}
		break
	}
	logger.Debug("%v exp:%v lvl:%v", b.GetPlayerId(), b.Data.Exp, b.Data.Level)
	if oldLevel != b.Data.Level {
		// 玩家等级更新时,自动接任务
		for i := oldLevel + 1; i <= b.Data.Level; i++ {
			b.GetPlayer().GetQuest().WhenPlayerLevelup(i)
		}
		b.GetPlayer().FireConditionEvent(&pb.EventPlayerProperty{
			PlayerId: b.GetPlayerId(),
			Property: "Level",
			Delta:    b.Data.Level - oldLevel,
			Current:  b.Data.Level,
		})
	}
	// 修改了需要保存的数据后,必须设置标记
	b.SetDirty()
	b.SyncDataToClient() // 同步数据给客户端
}

func (b *BaseInfo) TriggerPlayerExit(event *internal.EventPlayerExit) {
	b.Data.TotalOnlineSeconds += b.GetOnlineSecondsThisTime()
	b.Data.LastLogoutTimestamp = b.GetPlayer().GetTimerEntries().Now().Unix()
	b.SetDirty()
}

// 本次登录在线时长
func (b *BaseInfo) GetOnlineSecondsThisTime() int32 {
	now := b.GetPlayer().GetTimerEntries().Now().Unix()
	var onlineSeconds int32
	if b.Data.LastLoginTimestamp > 0 && now > b.Data.LastLoginTimestamp {
		onlineSeconds = int32(now - b.Data.LastLoginTimestamp)
	}
	return onlineSeconds
}

// 总在线时长
func (b *BaseInfo) GetTotalOnlineSeconds() int32 {
	return b.Data.TotalOnlineSeconds + b.GetOnlineSecondsThisTime()
}

// 生成一个新的重连session,写入数据并标记保存
func (b *BaseInfo) GenerateReconnectSession() {
	b.Data.ReconnectSession = fmt.Sprintf("%v", time.Now().UnixNano())
	b.SetDirty()
}

// 校验重连session是否有效
func (b *BaseInfo) VerifyReconnectSession(session string) bool {
	if session == "" {
		return false
	}
	return b.Data.ReconnectSession == session
}
