package cache

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
)

// Redis keys
const (
	maintenanceKey      = "maintenance:status"
	whitelistAccountKey = "whitelist:account"
	maintenanceUpdateChannel = "notify:maintenance_change"
)

// ---- 维护状态 ----

// maintenanceMode 维护状态的进程内副本
// 由InitMaintenanceCache负责加载与刷新,IsMaintenanceMode只读本副本,常态零Redis查询
var maintenanceMode atomic.Bool

// maintenanceCacheInitOnce 保证InitMaintenanceCache只执行一次初始化
// (订阅协程不能因多个组件重复调用而重复启动)
var maintenanceCacheInitOnce sync.Once

// InitMaintenanceCache 初始化维护状态内存缓存
// 各查询维护状态的服务器进程在Redis初始化完成后调用一次:
//  1. 同步读一次Redis,保证初始值正确
//  2. 订阅变更通知:GM设置维护/人工刷新后毫秒级刷新副本
//
// 重复调用安全(幂等):初始化逻辑只执行一次
//
// 通知语义为at-most-once(与region_notify一致):订阅断线窗口内发布的通知会永久丢失。
// 人工补偿——NotifyMaintenanceRefresh重新广播一次信号,所有订阅进程立即重读Redis
func InitMaintenanceCache(ctx context.Context) {
	maintenanceCacheInitOnce.Do(func() {
		syncMaintenanceMode()
		// payload仅为变更信号,不携带值:发布侧"写Redis"与"publish"是两步,
		// 信号+重读的组合保证副本永远以Redis权威值为准,不存在两步间的原子性问题
		SubscribeChannel(ctx, maintenanceUpdateChannel, func(payload string) {
			syncMaintenanceMode()
		})
	})
}

// syncMaintenanceMode 从Redis读取维护状态并更新内存副本
// Redis异常时保持副本当前值不变:不能默认翻转为false,否则Redis抖动期间
// 维护拦截会间歇性失效(比暂时保持旧值更危险);初始化阶段副本默认false,
// 与原直查实现"Redis错误fail-open返回false"的语义一致(可用性优先)
func syncMaintenanceMode() {
	val, err := GetRedis().Get(context.Background(), maintenanceKey).Result()
	if IsRedisError(err) {
		slog.Error("syncMaintenanceMode redis error, keep cached value", "error", err)
		return
	}
	maintenanceMode.Store(val == "1")
}

// IsMaintenanceMode 检查是否处于维护状态(读内存副本,零Redis查询)
// 数据来源:启动加载+PubSub变更通知,见InitMaintenanceCache;
func IsMaintenanceMode() bool {
	return maintenanceMode.Load()
}

// NotifyMaintenanceRefresh 广播一次维护状态刷新信号(不修改Redis中的值)
func NotifyMaintenanceRefresh() error {
	return PublishChannel(context.Background(), maintenanceUpdateChannel, "refresh")
}

// SetMaintenance 设置维护状态
// 写Redis权威值成功后广播变更信号
func SetMaintenance(on bool) error {
	ctx := context.Background()
	if on {
		if err := GetRedis().Set(ctx, maintenanceKey, "1", 0).Err(); err != nil {
			return err
		}
	} else {
		if err := GetRedis().Del(ctx, maintenanceKey).Err(); err != nil {
			return err
		}
	}
	if err := PublishChannel(ctx, maintenanceUpdateChannel, "update"); err != nil {
		slog.Warn("SetMaintenance publish notify failed, manual refresh required to converge", "error", err)
	}
	return nil
}

// ---- 白名单 - 账号 ----

// IsWhitelistedAccount 检查账号是否在白名单
// NOTE:Redis错误时fail-open返回false(可用性优先),需打Error日志可感知
func IsWhitelistedAccount(accountId int64) bool {
	ok, err := GetRedis().SIsMember(context.Background(), whitelistAccountKey, strconv.FormatInt(accountId, 10)).Result()
	if IsRedisError(err) {
		slog.Error("IsWhitelistedAccount redis error, whitelist check bypassed(fail-open)", "accountId", accountId, "error", err)
		return false
	}
	return ok
}

// AddWhitelistAccount 添加账号到白名单
func AddWhitelistAccount(accountId int64) error {
	return GetRedis().SAdd(context.Background(), whitelistAccountKey, strconv.FormatInt(accountId, 10)).Err()
}

// RemoveWhitelistAccount 从白名单删除账号
func RemoveWhitelistAccount(accountId int64) error {
	return GetRedis().SRem(context.Background(), whitelistAccountKey, strconv.FormatInt(accountId, 10)).Err()
}

// ---- 白名单查询 ----

// WhitelistEntry 白名单条目
type WhitelistEntry struct {
	Type  string `json:"Type"`  // "account"
	Value string `json:"Value"` // accountId字符串
}

// GetWhitelist 获取白名单列表(分页)
// whitelistType: "all"/"account"
// 返回 total, list, error
func GetWhitelist(page, size int, whitelistType string) (int64, []WhitelistEntry, error) {
	ctx := context.Background()
	accounts, err := GetRedis().SMembers(ctx, whitelistAccountKey).Result()
	if err != nil {
		return 0, nil, err
	}

	entries := make([]WhitelistEntry, 0, len(accounts))
	for _, acc := range accounts {
		entries = append(entries, WhitelistEntry{Type: "account", Value: acc})
	}

	total := int64(len(entries))
	// 分页
	start := (page - 1) * size
	if start >= len(entries) {
		return total, []WhitelistEntry{}, nil
	}
	end := start + size
	if end > len(entries) {
		end = len(entries)
	}
	return total, entries[start:end], nil
}
