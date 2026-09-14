package game

import (
	"log/slog"
	"sync"

	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
)

// offlineProcessing 离线玩家处理中的playerId集合(进程内标记)
//
// 为什么需要标记:OfflinePlayerProcess持有onlineaccount独占期间,该记录的特征
// (属于本服+玩家不在内存+无onlineplayer记录)与"进游崩溃残留"完全同构,
var offlineProcessing sync.Map

// IsOfflinePlayerProcessing 该玩家是否正处于离线数据处理中(本进程内)
func IsOfflinePlayerProcessing(playerId int64) bool {
	_, ok := offlineProcessing.Load(playerId)
	return ok
}

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
	// 标记离线处理中(必须在独占成功后、加载玩家数据前设置)
	offlineProcessing.Store(playerId, struct{}{})
	defer offlineProcessing.Delete(playerId)
	defer cache.RemoveOnlineAccount(accountId, playerId, gentity.GetApplication().GetId())
	if has, _ := db.GetPlayerDb().FindEntityById(playerId, data); has {
		return f(playerId, data)
	}
	slog.Debug("OfflinePlayerProcess not find data", "playerId", playerId, "accountId", accountId)
	return false
}
