package cache

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"

	"github.com/fish-tennis/gserver/util"
	"github.com/redis/go-redis/v9"
)

// game_time_cache.go 虚拟游戏时钟的组级偏移量存储与同步
//
// 背景:util.GameNow()=time.Now()+进程内偏移量(秒),见util/game_time.go。
// 游戏业务时间(每日/每周重置、限时道具、离线收益、活动排期)走GameNow实现"时间快进",
// 系统时间保持真实(支付回调/token验签等依赖真实墙钟的场景不受影响)。
// 启动加载一次 + 订阅变更通知,把Redis值刷进util.SetGameTimeOffset
const gameTimeOffsetKey = "gametime:{offset}"

// gameTimeUpdateChannel 偏移量变更通知频道
const gameTimeUpdateChannel = "notify:gametime_change"

// gameTimeSyncOnce 保证StartGameTimeSync只执行一次初始化
var gameTimeSyncOnce sync.Once

var (
	gameTimeChangeCallbacksMu sync.Mutex
	// gameTimeChangeCallbacks 已注册的偏移量变更回调
	gameTimeChangeCallbacks []func()
)

// RegisterGameTimeChangeCallback 注册"偏移量变更后"回调
func RegisterGameTimeChangeCallback(cb func()) {
	gameTimeChangeCallbacksMu.Lock()
	defer gameTimeChangeCallbacksMu.Unlock()
	gameTimeChangeCallbacks = append(gameTimeChangeCallbacks, cb)
}

// notifyGameTimeChange 偏移刷新成功后依次触发全部已注册的变更回调
func notifyGameTimeChange() {
	gameTimeChangeCallbacksMu.Lock()
	callbacks := make([]func(), len(gameTimeChangeCallbacks))
	copy(callbacks, gameTimeChangeCallbacks)
	gameTimeChangeCallbacksMu.Unlock()
	for _, cb := range callbacks {
		SafeInvoke("GameTimeChangeCallback", cb)
	}
}

// GetGameTimeOffset 从Redis读取组级游戏时间偏移量(秒)
// key不存在视为0:生产环境从不设置偏移,key本就不存在,0即"与真实时间一致"的正确语义
func GetGameTimeOffset() (int64, error) {
	val, err := GetRedis().Get(context.Background(), gameTimeOffsetKey).Result()
	if err == redis.Nil {
		// key不存在视为0:生产环境从不设置偏移,key本就不存在,
		// 0即"与真实时间一致"的正确语义
		return 0, nil
	}
	if IsRedisError(err) {
		// 走到这里才是真正的异常(redis.Nil已在上分支排除)
		return 0, err
	}
	sec, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("GetGameTimeOffset: invalid value %q: %w", val, err)
	}
	return sec, nil
}

// SetGameTimeOffset 设置组级游戏时间偏移量(秒),正值为时间快进
// 写Redis权威值成功后广播变更信号,组内已启动同步的进程毫秒级重读生效;
func SetGameTimeOffset(sec int64) error {
	ctx := context.Background()
	if err := GetRedis().Set(ctx, gameTimeOffsetKey, sec, 0).Err(); err != nil {
		return err
	}
	if err := PublishChannel(ctx, gameTimeUpdateChannel, "update"); err != nil {
		slog.Warn("SetGameTimeOffset publish notify failed", "error", err)
	}
	return nil
}

// syncGameTimeOffset 从Redis读取偏移量并刷进util进程内副本
func syncGameTimeOffset() bool {
	sec, err := GetGameTimeOffset()
	if err != nil {
		slog.Error("syncGameTimeOffset redis error, keep cached offset", "error", err)
		return false
	}
	util.SetGameTimeOffset(sec)
	return true
}

// onGameTimeUpdate 偏移量变更通知的订阅回调:重读Redis权威值
func onGameTimeUpdate(payload string) {
	if syncGameTimeOffset() {
		notifyGameTimeChange()
	}
}

// StartGameTimeSync 启动游戏时间偏移量的进程内同步
func StartGameTimeSync(ctx context.Context) {
	gameTimeSyncOnce.Do(func() {
		syncGameTimeOffset()
		// 订阅变更通知:信号到达即重读Redis权威值,成功后触发变更回调
		SubscribeChannel(ctx, gameTimeUpdateChannel, onGameTimeUpdate)
	})
}
