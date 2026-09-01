package cache

import (
	"context"
	"strconv"
)

// Redis keys
const (
	maintenanceKey      = "maintenance:status"
	whitelistAccountKey = "whitelist:account"
)

// ---- 维护状态 ----

// IsMaintenanceMode 检查是否处于维护状态
func IsMaintenanceMode() bool {
	val, err := GetRedis().Get(context.Background(), maintenanceKey).Result()
	if IsRedisError(err) {
		return false
	}
	return val == "1"
}

// SetMaintenance 设置维护状态
func SetMaintenance(on bool) error {
	ctx := context.Background()
	if on {
		return GetRedis().Set(ctx, maintenanceKey, "1", 0).Err()
	}
	return GetRedis().Del(ctx, maintenanceKey).Err()
}

// ---- 白名单 - 账号 ----

// IsWhitelistedAccount 检查账号是否在白名单
func IsWhitelistedAccount(accountId int64) bool {
	ok, err := GetRedis().SIsMember(context.Background(), whitelistAccountKey, strconv.FormatInt(accountId, 10)).Result()
	if IsRedisError(err) {
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
