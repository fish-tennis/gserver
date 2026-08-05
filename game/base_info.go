package game

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
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
				Level:           1,
				Exp:             0,
				CreateTimestamp: time.Now().Unix(),
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
	slog.Debug("BaseInfo.IncExp", "playerId", b.GetPlayerId(), "exp", b.Data.Exp, "level", b.Data.Level)
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
// 使用 crypto/rand 生成不可预测的随机 token,防止攻击者通过时间戳猜测 session 伪造重连
func (b *BaseInfo) GenerateReconnectSession() {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand 失败极为罕见(通常仅系统熵池耗尽),回退到时间戳保证功能可用
		slog.Error("GenerateReconnectSession rand failed", "pid", b.GetPlayer().GetId(), "err", err)
		buf = make([]byte, 16)
		// 使用 TimerEntries.Now() 保持确定性,不引入 system.Random
		nano := b.GetPlayer().GetTimerEntries().Now().UnixNano()
		for i := 0; i < 8; i++ {
			buf[i] = byte(nano >> (i * 8))
		}
	}
	b.Data.ReconnectSession = hex.EncodeToString(buf)
	b.SetDirty()
}

// 校验重连session是否有效
func (b *BaseInfo) VerifyReconnectSession(session string) bool {
	if session == "" {
		return false
	}
	return b.Data.ReconnectSession == session
}

// GetCreateDayCount 返回建号天数(建号当天返回1)
// 使用自然日计算,跨过0点即增加一天
func (b *BaseInfo) GetCreateDayCount() int32 {
	// 老玩家没有创角时间戳时,容错返回1
	if b.Data.CreateTimestamp <= 0 {
		return 1
	}
	now := b.GetPlayer().GetTimerEntries().Now()
	createTime := time.Unix(b.Data.CreateTimestamp, 0)
	// DayCount 返回两个日期相隔的自然日天数,建号当天为 0,所以 +1
	return int32(util.DayCount(now, createTime) + 1)
}
