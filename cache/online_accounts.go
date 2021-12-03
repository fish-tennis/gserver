package cache

import (
	"context"
	"fmt"
)

// 添加一个在线账号
func AddOnlineAccount(accountId int64, playerId int64) bool {
	ok, err := GetRedis().SetNX(context.TODO(), fmt.Sprintf("onlineaccount:%v", accountId), playerId, 0).Result()
	if IsRedisError(err) {
		return false
	}
	return ok
}

// 移除一个在线账号
func RemoveOnlineAccount(accountId int64) bool {
	_,err := GetRedis().Del(context.TODO(), fmt.Sprintf("onlineaccount:%v", accountId)).Result()
	if IsRedisError(err) {
		return false
	}
	return true
}

// 获取在线账号对应的玩家id
// 返回0表示账号不在线
func GetOnlineAccount(accountId int64) int64 {
	playerId,err := GetRedis().Get(context.TODO(), fmt.Sprintf("onlineaccount:%v", accountId)).Int64()
	if IsRedisError(err) {
		return 0
	}
	return playerId
}