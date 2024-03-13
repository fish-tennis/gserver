package internal

import "time"

// 玩家进游戏事件
type EventPlayerEntryGame struct {
	IsReconnect    bool
	OfflineSeconds int32 // 离线时长
}

// 玩家退出游戏
type EventPlayerExit struct {
}

// 日期更新
type EventDateChange struct {
	OldDate time.Time
	CurDate time.Time
}
