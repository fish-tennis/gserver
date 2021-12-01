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