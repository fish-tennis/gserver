package util

import (
	"sync/atomic"
	"time"
)

// gameTimeOffsetSec 游戏时间相对系统时间的偏移量(秒)
var gameTimeOffsetSec atomic.Int64

// GameNow 返回加上偏移量后的"游戏业务时间"
//
// 为什么需要它(时间解耦的背景):
//   - 测试环境需要验证"挂机1天后收益"这类长周期玩法,真实等待不可接受
//   - 不能通过修改云服务器系统时间来快进: 同一台云服务器上部署了多组服务器,
//     改系统时间会互相影响,且会导致 SDK 登录验签失败(验签依赖真实时间戳)
//   - 因此引入组级时间偏移量,只让"游戏业务时间"快进,系统时间保持真实
//
// 使用约定(白名单):
//   - 只有游戏业务逻辑时间(每日/每周重置、限时道具、离线收益、活动排期等)使用 GameNow
//   - 以下场景必须继续使用 time.Now(),不得改用 GameNow:
//     支付回调、token/会话有效期、心跳超时、日志时间戳、性能计时(耗时统计)
//     —— 这些依赖真实墙钟,若被偏移量污染会导致回调验签失败、会话错乱、监控失真
//
// 生产环境偏移量恒为 0,此时 GameNow 与 time.Now 完全一致,行为无任何变化
func GameNow() time.Time {
	return time.Now().Add(time.Duration(gameTimeOffsetSec.Load()) * time.Second)
}

// GameNowUnix 返回游戏业务时间的秒级 Unix 时间戳
func GameNowUnix() int64 {
	return GameNow().Unix()
}

// SetGameTimeOffset 设置游戏时间偏移量(秒),正值为时间快进
// 注意: 偏移量是"相对系统时间"的,设置后系统时间自然前进时游戏时间同步前进
//
// 时间回退风险提示: 若传入负偏移(把游戏时间调回过去),业务侧基于
// "GameNow - 上次结算时间"计算出的离线时长/冷却剩余可能为负数,
// 需要业务侧对负时长做防御(如按 0 处理),否则可能出现收益倒扣等异常
func SetGameTimeOffset(sec int64) {
	gameTimeOffsetSec.Store(sec)
}

// GameTimeOffset 读取当前游戏时间偏移量(秒)
func GameTimeOffset() int64 {
	return gameTimeOffsetSec.Load()
}
