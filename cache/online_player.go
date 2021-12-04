package cache

import (
	"context"
	"fmt"
)

// 添加一个在线玩家
// 缓存玩家和游戏服的对应关系,这样在分布式系统里,可以知道某个玩家当前在哪一台gameServer上
func AddOnlinePlayer(playerId int64, gameServerId int32) bool {
	ok, err := GetRedis().SetNX(context.Background(), fmt.Sprintf("onlineplayer:%v", playerId), gameServerId, 0).Result()
	if IsRedisError(err) {
		return false
	}
	return ok
}

// 移除一个在线玩家
func RemoveOnlinePlayer(playerId int64) bool {
	_,err := GetRedis().Del(context.Background(), fmt.Sprintf("onlineplayer:%v", playerId)).Result()
	if IsRedisError(err) {
		return false
	}
	return true
}

// 获取一个在线玩家当前所在的游戏服id
func GetOnlinePlayerGameServerId(playerId int64) int32 {
	gameServerId,err := GetRedis().Get(context.Background(), fmt.Sprintf("onlineplayer:%v", playerId)).Int()
	if IsRedisError(err) {
		return 0
	}
	return int32(gameServerId)
}