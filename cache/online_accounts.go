package cache

import (
	"context"
	"strconv"
	"strings"

	"github.com/fish-tennis/gentity/util"
	"github.com/redis/go-redis/v9"
)

func keyOnlineAccount(accountId int64) string {
	return "onlineaccount:" + strconv.FormatInt(accountId, 10)
}

// 条件释放在线账号记录:值等于 "playerId;gameServerId" 时才删除
// 防止基于旧快照的清理误删其他流程已写入的新记录
// (如:清理请求因网络延迟晚到时,该账号可能已在别的游戏服重新上线)
// KEYS[1]: onlineaccount:{accountId} 账号独占记录(string)
// ARGV[1]: 期望值,格式"playerId;gameServerId"(仅当前值等于它才删除)
// 返回: 1=已删除 0=值不匹配未删除
var luaReleaseOnlineAccountIfValue = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
	return redis.call('DEL', KEYS[1])
end
return 0
`)

// 添加一个在线账号
// SetNX单命令天然原子,实现账号的"独占性"(一个账号同时只能在一个游戏服上登录)
func AddOnlineAccount(accountId int64, playerId int64, gameServerId int32) bool {
	val := strconv.FormatInt(playerId, 10) + ";" + strconv.FormatInt(int64(gameServerId), 10)
	ok, err := GetRedis().SetNX(context.Background(), keyOnlineAccount(accountId), val, 0).Result()
	if IsRedisError(err) {
		return false
	}
	return ok
}

// 移除一个在线账号
// 条件释放:仅当记录仍为指定的 playerId+gameServerId 时才删除
// 防止误删:释放请求因网络延迟晚到时,账号可能已被新的登录流程重新写入
func RemoveOnlineAccount(accountId int64, playerId int64, gameServerId int32) bool {
	expectVal := strconv.FormatInt(playerId, 10) + ";" + strconv.FormatInt(int64(gameServerId), 10)
	_, err := runGserverFunction(funcReleaseOnlineAccount,
		[]string{keyOnlineAccount(accountId)}, expectVal)
	if IsRedisError(err) {
		return false
	}
	return true
}

// 获取在线账号对应的玩家id
// 返回0表示账号不在线
func GetOnlineAccount(accountId int64) (playerId int64, gameServerId int32) {
	playerIdAndGameServerId, err := GetRedis().Get(context.Background(), keyOnlineAccount(accountId)).Result()
	if IsRedisError(err) {
		return
	}
	if len(playerIdAndGameServerId) == 0 {
		return
	}
	ids := strings.Split(playerIdAndGameServerId, ";")
	if len(ids) != 2 {
		return
	}
	playerId = util.Atoi64(ids[0])
	gameServerId = int32(util.Atoi(ids[1]))
	return
}
