package cache

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	"github.com/fish-tennis/gentity/util"
	"github.com/redis/go-redis/v9"
)

func keyOnlinePlayer(playerId int64) string {
	return "onlineplayer:" + strconv.FormatInt(playerId, 10)
}

func keyGameServerPlayer(gameServerId int32) string {
	return "game:" + strconv.FormatInt(int64(gameServerId), 10)
}

// 在线玩家的key结构设计说明(生产环境为Redis Cluster多节点部署):
//
//	onlineplayer:{playerId} -> "accountId;gameServerId" 玩家所在服务器的权威记录
//	onlineaccount:{accountId} -> "playerId;gameServerId" 账号独占记录
//	game:{gameServerId} -> Set<playerId> 服务器宕机后的恢复索引(数据可丢失)
//
// 三个key的hash tag分别是playerId/accountId/gameServerId,在集群中大概率落在不同slot,
// 无法用一个Lua脚本原子操作,因此正确性设计为:
//  1. 每个key自身的读写用单key Lua脚本原子完成(条件占有/条件释放),集群兼容
//  2. game:{gsid}集合仅作为宕机恢复索引,允许残留数据,不参与正确性保证
//  3. 跨key的组合操作(SAdd+占有/释放账号+释放玩家)按崩溃安全的顺序分步执行,
//     任何一步的中间状态都可被启动修复(ResetOnlinePlayer)或登录兜底清理修复
//  4. 所有释放操作都是"条件释放"(值匹配才删):旧服务器延迟到达的下线清理
//     不会误删新服务器已写入的新记录,这是消除跨key竞态的关键

var (
	// 占有在线玩家记录:key不存在或记录属于本服(本服崩溃残留)时写入
	// KEYS[1]: onlineplayer:{playerId} 玩家所在服务器的权威记录(string)
	// ARGV[1]: 新值,格式"accountId;gameServerId"
	// ARGV[2]: 本服gameServerId(十进制字符串,与记录值分号后的部分比较)
	// 返回: 1=占有成功(含本服残留重入) 0=被其他服务器持有
	luaClaimOnlinePlayer = redis.NewScript(`
local cur = redis.call('GET', KEYS[1])
if cur == false then
	redis.call('SET', KEYS[1], ARGV[1])
	return 1
end
-- ';(.+)$': 取第一个分号后的全部内容,即记录值"accountId;gameServerId"中的gameServerId部分
local gsid = string.match(cur, ';(.+)$')
if gsid == ARGV[2] then
	redis.call('SET', KEYS[1], ARGV[1])
	return 1
end
return 0
`)

	// 条件释放在线玩家记录:记录的gameServerId等于本服时才删除
	// 防止旧服务器延迟到达的下线清理误删新服务器已写入的新记录
	// KEYS[1]: onlineplayer:{playerId}
	// ARGV[1]: 本服gameServerId(十进制字符串)
	// 返回: 1=已删除(含记录本就不存在) 0=记录属于其他服务器未删除
	luaReleaseOnlinePlayerIfServer = redis.NewScript(`
local cur = redis.call('GET', KEYS[1])
if cur == false then
	return 1
end
-- ';(.+)$': 取第一个分号后的全部内容,即记录值"accountId;gameServerId"中的gameServerId部分
local gsid = string.match(cur, ';(.+)$')
if gsid == ARGV[1] then
	redis.call('DEL', KEYS[1])
	return 1
end
return 0
`)

	// 接管在线玩家记录:原子地读取旧值并写入新值
	// 用于清理宕机服务器的残留记录:调用方已通过AddOnlineAccount获得账号独占,
	// 记录持有者只可能是已宕机服务器的残留或正在下线的旧服务器(其清理是条件释放,不会误删)
	// KEYS[1]: onlineplayer:{playerId}
	// ARGV[1]: 新值,格式"accountId;gameServerId"
	// 返回: 旧值字符串,格式"accountId;gameServerId"(记录不存在时返回空串)
	luaTakeOverOnlinePlayer = redis.NewScript(`
local cur = redis.call('GET', KEYS[1])
redis.call('SET', KEYS[1], ARGV[1])
if cur == false then
	return ''
end
return cur
`)
)

// 解析在线玩家记录 "accountId;gameServerId"
func parseOnlinePlayerValue(val string) (accountId int64, gameServerId int32) {
	if len(val) == 0 {
		return
	}
	ids := strings.Split(val, ";")
	if len(ids) != 2 {
		return
	}
	accountId = util.Atoi64(ids[0])
	gameServerId = int32(util.Atoi(ids[1]))
	return
}

// 添加一个在线玩家
// 缓存玩家和游戏服的对应关系,这样在分布式系统里,可以知道某个玩家当前在哪一台gameServer上
func AddOnlinePlayer(playerId, accountId int64, gameServerId int32) bool {
	val := strconv.FormatInt(accountId, 10) + ";" + strconv.FormatInt(int64(gameServerId), 10)
	// 一个游戏服上的在线玩家缓存,用于在服务器宕机后的恢复操作
	// 当一个游戏服异常宕机时,在线玩家(keyOnlinePlayer)缓存没能清除,将导致这部分玩家不能正常登录游戏
	// 所有游戏服务器需要把该服务器上的玩家记录在缓存中,当宕机重启后,游戏服会修复这部分玩家的缓存数据
	// 这里算是一个BigKey,假设一个游戏服进程最多承载5000个在线玩家,那么整个key的大小:5000*sizeof(int64)=40K,还是可以接受的
	// 先 SAdd 再占有:若两步之间崩溃,SAdd 的残留会被 ResetOnlinePlayer 修复;
	// 若先占有再 SAdd,崩溃窗口会导致 keyOnlinePlayer 残留且不在任何 game 集合中,只能依赖登录兜底清理
	_, err := GetRedis().SAdd(context.Background(), keyGameServerPlayer(gameServerId), playerId).Result()
	if IsRedisError(err) {
		slog.Error("AddOnlinePlayer error", "error", err)
		return false
	}
	// 原子占有(优先FCALL,回退EVAL):不存在或本服残留时写入
	// 相比SetNX,额外支持"本服崩溃残留"场景:重启后可直接重新占有自己的记录
	res, err := runGserverFunction(funcClaimOnlinePlayer,
		[]string{keyOnlinePlayer(playerId)}, val, strconv.FormatInt(int64(gameServerId), 10))
	if IsRedisError(err) {
		// Redis异常才回滚 SAdd
		GetRedis().SRem(context.Background(), keyGameServerPlayer(gameServerId), playerId)
		slog.Error("AddOnlinePlayer error", "error", err)
		return false
	}
	ok := res == int64(1)
	if !ok {
		// 占有失败说明记录被其他服务器持有:不回滚SAdd!
		// 1. 调用方(GameServer.AddPlayer)此时会强制接管(TakeOverOnlinePlayer)记录为本服,
		//    接管后记录与本服集合恰好一致;若此处回滚SRem,接管后将出现
		//    "记录指向本服但本服集合无此玩家"的状态,服务器宕机重启后
		//    ResetOnlinePlayer无法从集合索引中找到该玩家,自修复能力退化
		// 2. 若调用方不接管,集合残留也只是恢复索引的冗余项,
		//    ResetOnlinePlayer会按记录的实际归属正确跳过,无害
	}
	return ok
}

// 接管一个在线玩家记录,返回旧记录的 accountId 和 gameServerId
// 仅在已通过 AddOnlineAccount 获得账号独占后使用(见 luaTakeOverOnlinePlayer 的说明)
func TakeOverOnlinePlayer(playerId, accountId int64, gameServerId int32) (oldAccountId int64, oldGameServerId int32) {
	val := strconv.FormatInt(accountId, 10) + ";" + strconv.FormatInt(int64(gameServerId), 10)
	res, err := runGserverFunction(funcTakeOverOnlinePlayer, []string{keyOnlinePlayer(playerId)}, val)
	if IsRedisError(err) {
		slog.Error("TakeOverOnlinePlayer error", "playerId", playerId, "error", err)
		return
	}
	oldVal, _ := res.(string)
	return parseOnlinePlayerValue(oldVal)
}

// 移除一个在线玩家
// 条件释放:仅当记录仍属于本服时才删除,防止误删其他服务器的新记录
func RemoveOnlinePlayer(playerId int64, gameServerId int32) bool {
	_, err := runGserverFunction(funcReleaseOnlinePlayer,
		[]string{keyOnlinePlayer(playerId)}, strconv.FormatInt(int64(gameServerId), 10))
	if IsRedisError(err) {
		slog.Error("RemoveOnlinePlayer error", "playerId", playerId, "error", err)
		return false
	}
	// 从本服集合移除:若pid是本服集合中的残留数据(玩家实际在其他服务器),移除也是正确的清理
	_, err = GetRedis().SRem(context.Background(), keyGameServerPlayer(gameServerId), playerId).Result()
	if IsRedisError(err) {
		slog.Error("RemoveOnlinePlayer error", "playerId", playerId, "error", err)
	}
	return true
}

// 重置一个服务器上的在线玩家缓存
// 服务器宕机重启后调用:修复本服宕机时残留的在线记录
func ResetOnlinePlayer(gameServerId int32, repairFunc func(playerId, accountId int64) error) {
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
			accountId, curGameServerId := GetOnlinePlayer(playerId)
			// 只清理仍属于本服的残留记录
			// 记录不存在:已被正常下线清理,无需处理
			// 记录属于其他服务器:本服集合中的该元素是历史残留(SAdd后占有失败的残留数据),
			// 不能动其他服务器的在线记录,否则会把在线玩家"踢下线"
			if accountId <= 0 || curGameServerId != gameServerId {
				slog.Debug("ResetOnlinePlayer skip non-local record",
					"playerId", playerId, "curGameServerId", curGameServerId, "gameServerId", gameServerId)
				continue
			}
			if repairFunc != nil {
				repairFunc(playerId, accountId)
			}
			RemoveOnlinePlayer(playerId, gameServerId)
			RemoveOnlineAccount(accountId, playerId, gameServerId)
			slog.Debug("ResetOnlinePlayer repair", "playerId", playerId, "accountId", accountId, "gameServerId", gameServerId)
		}
	}
}

// 获取一个在线玩家当前所在的游戏服id
func GetOnlinePlayer(playerId int64) (accountId int64, gameServerId int32) {
	accountIdAndGameServerId, err := GetRedis().Get(context.Background(), keyOnlinePlayer(playerId)).Result()
	if IsRedisError(err) {
		return
	}
	return parseOnlinePlayerValue(accountIdAndGameServerId)
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
		_, gameServerId := parseOnlinePlayerValue(val)
		if gameServerId > 0 {
			serverMap[playerIds[i]] = gameServerId
		}
	}
	return serverMap
}
