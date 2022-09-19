package game

import (
	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
)

// 对离线玩家的数据处理
func OfflinePlayerProcess(playerId int64, data interface{}, f func(offlinePlayerId int64, offlineData interface{}) bool) bool {
	accountId,_ := db.GetPlayerDb().FindAccountIdByPlayerId(playerId)
	logger.Debug("OfflinePlayerProcess playerId:%v accountId:%v", playerId, accountId)
	if accountId == 0 {
		return false
	}
	// 防止离线数据处理期间,玩家上线,导致数据覆盖
	if !cache.AddOnlineAccount(accountId, playerId, internal.GetServer().GetServerId()) {
		logger.Debug("OfflinePlayerProcess AddOnlineAccount failed playerId:%v accountId:%v", playerId, accountId)
		return false
	}
	defer cache.RemoveOnlineAccount(accountId)
	if has, _ := db.GetPlayerDb().FindEntityById(playerId, data); has {
		return f(playerId, data)
	}
	logger.Debug("OfflinePlayerProcess not find data playerId:%v accountId:%v", playerId, accountId)
	return false
}