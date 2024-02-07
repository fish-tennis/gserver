package internal

import "time"

// 玩家进游戏事件
type EventPlayerEntryGame struct {
	IsReconnect bool
}

// 日期更新
type EventDateChange struct {
	OldDate time.Time
	CurDate time.Time
}