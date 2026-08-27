package game

import (
	"log/slog"

	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
)

// 对离线玩家的数据处理
// NOTE:当对离线玩家进行数据修改时,需要考虑并发问题,比如多个协程都在对同一个玩家进行数据修改
// 或者该玩家正在上线过程中
func OfflinePlayerProcess(playerId int64, data interface{}, f func(offlinePlayerId int64, offlineData interface{}) bool) bool {
	accountId, _ := db.GetPlayerDb().FindAccountIdByPlayerId(playerId)
	slog.Debug("OfflinePlayerProcess", "playerId", playerId, "accountId", accountId)
	if accountId == 0 {
		return false
	}
	// 防止离线数据处理期间,玩家上线,导致数据覆盖
	if !cache.AddOnlineAccount(accountId, playerId, gentity.GetApplication().GetId()) {
		slog.Debug("OfflinePlayerProcess AddOnlineAccount failed", "playerId", playerId, "accountId", accountId)
		return false
	}
	defer cache.RemoveOnlineAccount(accountId, playerId, gentity.GetApplication().GetId())
	if has, _ := db.GetPlayerDb().FindEntityById(playerId, data); has {
		return f(playerId, data)
	}
	slog.Debug("OfflinePlayerProcess not find data", "playerId", playerId, "accountId", accountId)
	return false
}
