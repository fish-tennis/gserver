package cache

import (
	"context"
	"log/slog"
	"github.com/redis/go-redis/v9"
	"strconv"
	"strings"

	"github.com/fish-tennis/gentity/util"
)

func keyOnlinePlayer(playerId int64) string {
	return "onlineplayer:" + strconv.FormatInt(playerId, 10)
}

func keyGameServerPlayer(gameServerId int32) string {
	return "game:" + strconv.FormatInt(int64(gameServerId), 10)
}

// 添加一个在线玩家
// 缓存玩家和游戏服的对应关系,这样在分布式系统里,可以知道某个玩家当前在哪一台gameServer上
func AddOnlinePlayer(playerId,accountId int64, gameServerId int32) bool {
	val := strconv.FormatInt(accountId, 10) + ";" + strconv.FormatInt(int64(gameServerId), 10)
	// 一个游戏服上的在线玩家缓存,用于在服务器宕机后的恢复操作
	// 当一个游戏服异常宕机时,在线玩家(keyOnlinePlayer)缓存没能清除,将导致这部分玩家不能正常登录游戏
	// 所有游戏服务器需要把该服务器上的玩家记录在缓存中,当宕机重启后,游戏服会修复这部分玩家的缓存数据
	// 这里算是一个BigKey,假设一个游戏服进程最多承载5000个在线玩家,那么整个key的大小:5000*sizeof(int64)=40K,还是可以接受的
	// 先 SAdd 再 SetNX:若两步之间崩溃,SAdd 的残留会被 ResetOnlinePlayer 修复;
	// 若先 SetNX 再 SAdd,崩溃窗口会导致 keyOnlinePlayer 永久残留且无法被修复 -> 玩家永久无法登录
	_, err := GetRedis().SAdd(context.Background(), keyGameServerPlayer(gameServerId), playerId).Result()
	if IsRedisError(err) {
		slog.Error("AddOnlinePlayer error", "error", err)
		return false
	}
	ok, err := GetRedis().SetNX(context.Background(), keyOnlinePlayer(playerId), val, 0).Result()
	if IsRedisError(err) {
		// Redis异常才回滚 SAdd
		GetRedis().SRem(context.Background(), keyGameServerPlayer(gameServerId), playerId)
		slog.Error("AddOnlinePlayer error", "error", err)
		return false
	}
	// 注意: SetNX 返回 false(玩家已在线)时,不能执行 SRem 回滚!
	// 因为传入的 gameServerId 可能和玩家当前所在的 gameServerId 相同,
	// 此时 SRem 会从正确的集合中误删 pid,导致 keyOnlinePlayer 存在但不在任何
	// keyGameServerPlayer 集合中,ResetOnlinePlayer 无法清理 -> 玩家永久无法登录。
	// SAdd 是幂等操作,残留一个多余的集合元素不会导致永久无法登录,风险可接受。
	return ok
}

// 移除一个在线玩家
func RemoveOnlinePlayer(playerId int64, gameServerId int32) bool {
	_, err := GetRedis().Del(context.Background(), keyOnlinePlayer(playerId)).Result()
	if IsRedisError(err) {
		return false
	}
	_, err = GetRedis().SRem(context.Background(), keyGameServerPlayer(gameServerId), playerId).Result()
	if IsRedisError(err) {
		slog.Error("RemoveOnlinePlayer error", "error", err)
	}
	return true
}

// 重置一个服务器上的在线玩家缓存
func ResetOnlinePlayer(gameServerId int32,repairFunc func(playerId,accountId int64) error) {
	for {
		playerIds, err := GetRedis().SPopN(context.Background(), keyGameServerPlayer(gameServerId), 128).Result()
		if IsRedisError(err) {
			break
		}
		if len(playerIds) == 0 {
			break
		}
		for _, v := range playerIds {
			playerId := util.Atoi64(v)
			accountId, _ := GetOnlinePlayer(playerId)
			if repairFunc != nil {
				repairFunc(playerId, accountId)
			}
			GetRedis().Del(context.Background(), keyOnlinePlayer(playerId))
			RemoveOnlineAccount(accountId)
			slog.Debug("ResetOnlinePlayer repair", "playerId", playerId, "accountId", accountId, "gameServerId", gameServerId)
		}
	}
}

// 获取一个在线玩家当前所在的游戏服id
func GetOnlinePlayer(playerId int64) (accountId int64, gameServerId int32) {
	accountIdAndGameServerId,err := GetRedis().Get(context.Background(), keyOnlinePlayer(playerId)).Result()
	if IsRedisError(err) {
		return
	}
	if len(accountIdAndGameServerId) == 0 {
		return
	}
	ids := strings.Split(accountIdAndGameServerId, ";")
	if len(ids) != 2 {
		return
	}
	accountId = util.Atoi64(ids[0])
	gameServerId = int32(util.Atoi(ids[1]))
	return
}

// 批量获取多个在线玩家当前所在的游戏服id
// 返回 playerId -> gameServerId 映射,不在线或查询失败的玩家不在返回值中
// 使用 Pipeline 而非 MGet:Pipeline 天然兼容 Redis Cluster(跨 slot),且只需一次网络往返
func GetOnlinePlayers(playerIds []int64) map[int64]int32 {
	if len(playerIds) == 0 {
		return nil
	}
	ctx := context.Background()
	pipe := GetRedis().Pipeline()
	cmds := make([]*redis.StringCmd, len(playerIds))
	for i, pid := range playerIds {
		cmds[i] = pipe.Get(ctx, keyOnlinePlayer(pid))
	}
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		slog.Error("GetOnlinePlayers Pipeline err", "error", err)
		// Pipeline 部分失败不影响已成功的,继续处理
	}
	serverMap := make(map[int64]int32, len(playerIds))
	for i, cmd := range cmds {
		val, err := cmd.Result()
		if err != nil { // redis.Nil 或其他错误,玩家不在线
			continue
		}
		ids := strings.Split(val, ";")
		if len(ids) != 2 {
			continue
		}
		serverMap[playerIds[i]] = int32(util.Atoi(ids[1]))
	}
	return serverMap
}