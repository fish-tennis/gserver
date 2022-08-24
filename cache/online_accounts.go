package cache

import (
	"context"
	"fmt"
	"github.com/fish-tennis/gentity/util"
	"strings"
)

func keyOnlineAccount(accountId int64) string {
	return fmt.Sprintf("onlineaccount:%v", accountId)
}

// 添加一个在线账号
func AddOnlineAccount(accountId int64, playerId int64, gameServerId int32) bool {
	val := fmt.Sprintf("%v;%v", playerId, gameServerId)
	ok, err := GetRedis().SetNX(context.Background(), keyOnlineAccount(accountId), val, 0).Result()
	if IsRedisError(err) {
		return false
	}
	return ok
}

// 移除一个在线账号
func RemoveOnlineAccount(accountId int64) bool {
	_,err := GetRedis().Del(context.Background(), keyOnlineAccount(accountId)).Result()
	if IsRedisError(err) {
		return false
	}
	return true
}

// 获取在线账号对应的玩家id
// 返回0表示账号不在线
func GetOnlineAccount(accountId int64) (playerId int64, gameServerId int32) {
	playerIdAndGameServerId,err := GetRedis().Get(context.Background(), fmt.Sprintf("onlineaccount:%v", accountId)).Result()
	if IsRedisError(err) {
		return
	}
	ids := strings.Split(playerIdAndGameServerId,";")
	if len(ids) != 2 {
		return
	}
	playerId = util.Atoi64(ids[0])
	gameServerId = int32(util.Atoi(ids[1]))
	return
}