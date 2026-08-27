package cache

import (
	"context"
	"log/slog"

	"github.com/fish-tennis/gentity"
	"github.com/redis/go-redis/v9"
)

// ==================== gserver的Redis Function库 ====================
// 在线玩家/在线账号的原子操作函数(Redis 7.0+)
// 优先FCALL执行;库未安装或Redis版本低于7.0时,自动回退到等价的EVAL脚本(见online_player.go/online_accounts.go)
// 函数体必须与EVAL回退脚本保持一致,修改时需同步修改两处
//
// NOTE:
//  1. Redis Cluster中FUNCTION不会跨节点同步,安装时会对所有master执行(gentity.InstallRedisFunctionLibrary)
//  2. 函数名只能包含字母/数字/下划线(Redis 7.x限制)
//  3. 所有函数均只操作KEYS[1]单个key,天然满足集群的slot约束
const gserverFunctionLibrarySource = `#!lua name=gserver

-- 占有在线玩家记录:key不存在或记录属于本服(本服崩溃残留)时写入
-- keys[1]: onlineplayer:{playerId} 玩家所在服务器的权威记录(string)
-- args[1]: 新值,格式"accountId;gameServerId"
-- args[2]: 本服gameServerId(十进制字符串,与记录值分号后的部分比较)
-- 返回: 1=占有成功(含本服残留重入) 0=被其他服务器持有
local function claim_online_player(keys, args)
	local cur = redis.call('GET', keys[1])
	if cur == false then
		redis.call('SET', keys[1], args[1])
		return 1
	end
	-- ';(.+)$': 取第一个分号后的全部内容,即记录值"accountId;gameServerId"中的gameServerId部分
	local gsid = string.match(cur, ';(.+)$')
	if gsid == args[2] then
		redis.call('SET', keys[1], args[1])
		return 1
	end
	return 0
end

-- 条件释放在线玩家记录:记录的gameServerId等于本服时才删除
-- 防止旧服务器延迟到达的下线清理误删新服务器已写入的新记录
-- keys[1]: onlineplayer:{playerId}
-- args[1]: 本服gameServerId(十进制字符串)
-- 返回: 1=已删除(含记录本就不存在) 0=记录属于其他服务器未删除
local function release_online_player(keys, args)
	local cur = redis.call('GET', keys[1])
	if cur == false then
		return 1
	end
	-- ';(.+)$': 取第一个分号后的全部内容,即记录值"accountId;gameServerId"中的gameServerId部分
	local gsid = string.match(cur, ';(.+)$')
	if gsid == args[1] then
		redis.call('DEL', keys[1])
		return 1
	end
	return 0
end

-- 接管在线玩家记录:原子地读取旧值并写入新值
-- 仅在已通过AddOnlineAccount获得账号独占后使用(记录持有者只可能是
-- 已宕机服务器的残留或正在下线的旧服务器,旧服务器的下线清理是条件释放,不会误删新记录)
-- keys[1]: onlineplayer:{playerId}
-- args[1]: 新值,格式"accountId;gameServerId"
-- 返回: 旧值字符串,格式"accountId;gameServerId"(记录不存在时返回空串)
local function takeover_online_player(keys, args)
	local cur = redis.call('GET', keys[1])
	redis.call('SET', keys[1], args[1])
	if cur == false then
		return ''
	end
	return cur
end

-- 条件释放在线账号记录:值等于"playerId;gameServerId"时才删除
-- 防止基于旧快照的清理误删其他流程已写入的新记录
-- (如:清理请求因网络延迟晚到时,该账号可能已在别的游戏服重新上线)
-- keys[1]: onlineaccount:{accountId} 账号独占记录(string)
-- args[1]: 期望值,格式"playerId;gameServerId"(仅当前值等于它才删除)
-- 返回: 1=已删除 0=值不匹配未删除
local function release_online_account(keys, args)
	if redis.call('GET', keys[1]) == args[1] then
		return redis.call('DEL', keys[1])
	end
	return 0
end

redis.register_function('gserver_claim_online_player', claim_online_player)
redis.register_function('gserver_release_online_player', release_online_player)
redis.register_function('gserver_takeover_online_player', takeover_online_player)
redis.register_function('gserver_release_online_account', release_online_account)
`

// gserver函数库中的函数名
const (
	funcClaimOnlinePlayer    = "gserver_claim_online_player"
	funcReleaseOnlinePlayer  = "gserver_release_online_player"
	funcTakeOverOnlinePlayer = "gserver_takeover_online_player"
	funcReleaseOnlineAccount = "gserver_release_online_account"
)

// gserver函数执行器:优先FCALL,不可用自动回退EVAL
// NOTE:随NewRedis一起重建(绑定当前redis客户端)
var gserverFuncRunner *gentity.FunctionRunner

// initGserverFunctions 初始化gserver函数执行器并尝试安装函数库
// 安装失败(Redis<7.0或测试用miniredis等)自动回退EVAL,功能不受影响
func initGserverFunctions() {
	client := GetRedis()
	gserverFuncRunner = gentity.NewFunctionRunner(client, map[string]*redis.Script{
		funcClaimOnlinePlayer:    luaClaimOnlinePlayer,
		funcReleaseOnlinePlayer:  luaReleaseOnlinePlayerIfServer,
		funcTakeOverOnlinePlayer: luaTakeOverOnlinePlayer,
		funcReleaseOnlineAccount: luaReleaseOnlineAccountIfValue,
	})
	if err := gentity.InstallRedisFunctionLibrary(context.Background(), client, gserverFunctionLibrarySource); err != nil {
		// go-redis的错误信息会携带整个库源码,截断后再打日志
		slog.Warn("gserver redis functions unavailable, fallback to EVAL", "error", truncateError(err, 120))
		return
	}
	gserverFuncRunner.MarkAvailable()
	slog.Debug("gserver redis functions installed, FCALL enabled")
}

// truncateError 截断错误信息(FUNCTION失败时go-redis会把整条命令参数拼进错误,非常长)
func truncateError(err error, max int) string {
	msg := err.Error()
	if len(msg) > max {
		return msg[:max] + "..."
	}
	return msg
}

// runGserverFunction 执行gserver的原子函数
// keys/args协议与EVAL回退脚本完全一致
func runGserverFunction(name string, keys []string, args ...interface{}) (interface{}, error) {
	return gserverFuncRunner.Run(name, keys, args...)
}
