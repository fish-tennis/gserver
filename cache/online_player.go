package cache

import (
	"context"
	"fmt"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/util"
	"strings"
)

func keyOnlinePlayer(playerId int64) string {
	return fmt.Sprintf("onlineplayer:%v", playerId)
}

func keyGameServerPlayer(gameServerId int32) string {
	return fmt.Sprintf("gameplayer:%v", gameServerId)
}

// 添加一个在线玩家
// 缓存玩家和游戏服的对应关系,这样在分布式系统里,可以知道某个玩家当前在哪一台gameServer上
func AddOnlinePlayer(playerId,accountId int64, gameServerId int32) bool {
	val := fmt.Sprintf("%v;%v", accountId, gameServerId)
	ok, err := GetRedis().SetNX(context.Background(), keyOnlinePlayer(playerId), val, 0).Result()
	if IsRedisError(err) {
		return false
	}
	// 一个游戏服上的在线玩家缓存,用于在服务器宕机后的恢复操作
	// 当一个游戏服异常宕机时,在线玩家(keyOnlinePlayer)缓存没能清除,将导致这部分玩家不能正常登录游戏
	// 所有游戏服务器需要把该服务器上的玩家记录在缓存中,当宕机重启后,游戏服会修复这部分玩家的缓存数据
	// 这里算是一个BigKey,假设一个游戏服进程最多承载5000个在线玩家,那么整个key的大小:5000*sizeof(int64)=40K,还是可以接受的
	_,err = GetRedis().SAdd(context.Background(), keyGameServerPlayer(gameServerId), playerId).Result()
	if IsRedisError(err) {
		logger.Error("%v", err)
	}
	return ok
}

// 移除一个在线玩家
func RemoveOnlinePlayer(playerId int64, gameServerId int32) bool {
	_,err := GetRedis().Del(context.Background(), keyOnlinePlayer(playerId)).Result()
	if IsRedisError(err) {
		return false
	}
	_,err = GetRedis().SRem(context.Background(), keyGameServerPlayer(gameServerId), playerId).Result()
	if IsRedisError(err) {
		logger.Error("%v", err)
	}
	return true
}

// 重置一个服务器上的在线玩家缓存
func ResetOnlinePlayer(gameServerId int32,repairFunc func(playerId,accountId int64) error) {
	for {
		playerIds,err := GetRedis().SPopN(context.Background(), keyGameServerPlayer(gameServerId), 128).Result()
		if IsRedisError(err) {
			break
		}
		if len(playerIds) == 0 {
			break
		}
		for _,v := range playerIds {
			playerId := util.Atoi64(v)
			accountId,_ := GetOnlinePlayer(playerId)
			if repairFunc != nil {
				repairFunc(playerId, accountId)
			}
			GetRedis().Del(context.Background(), keyOnlinePlayer(playerId))
			RemoveOnlineAccount(accountId)
			logger.Debug("repair:%v %v %v", playerId, accountId, gameServerId)
		}
	}
}

// 获取一个在线玩家当前所在的游戏服id
func GetOnlinePlayer(playerId int64) (accountId int64, gameServerId int32) {
	accountIdAndGameServerId,err := GetRedis().Get(context.Background(), fmt.Sprintf("onlineplayer:%v", playerId)).Result()
	if IsRedisError(err) {
		return
	}
	if len(accountIdAndGameServerId) == 0 {
		return
	}
	ids := strings.Split(accountIdAndGameServerId,";")
	if len(ids) != 2 {
		return
	}
	accountId = util.Atoi64(ids[0])
	gameServerId = int32(util.Atoi(ids[1]))
	return
}