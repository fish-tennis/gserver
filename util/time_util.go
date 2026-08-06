package util

import (
	"math"
	"time"

	"github.com/fish-tennis/gserver/pb"
)

// ServerLocation 服务器统一时区,分布式部署中所有节点必须一致
// 默认 UTC+8(中国标准时间),可通过覆盖此变量实现自定义时区
var ServerLocation = time.FixedZone("CST", 8*3600)

// 计算超时时间戳
func GetTimeoutTimestamp(timeType, timeout int32, now time.Time) int32 {
	switch timeType {
	case int32(pb.TimeType_TimeType_Timestamp):
		if timeout <= 0 {
			return 0
		}
		// 防溢出:int32 时间戳 + timeout 可能超过 MaxInt32(Y2038),钳制到上限
		result := int64(now.Unix()) + int64(timeout)
		if result > math.MaxInt32 {
			return math.MaxInt32
		}
		return int32(result)
	case int32(pb.TimeType_TimeType_Date):
		y := timeout / 10000
		m := (timeout / 100) % 100
		d := timeout % 100
		return int32(time.Date(int(y), time.Month(int(m)), int(d), 0, 0, 0, 0, ServerLocation).Unix())
	}
	return 0
}

// 去除Time中的时分秒,只保留日期
func ToDate(t time.Time) time.Time {
	y, m, d := t.In(ServerLocation).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, ServerLocation)
}

// 转换成20240219格式
func ToDateInt(t time.Time) int32 {
	y, m, d := t.In(ServerLocation).Date()
	return int32(y*10000 + int(m)*100 + d)
}

// 2个日期的相隔天数
func DayCount(a time.Time, b time.Time) int {
	aY, aM, aD := a.In(ServerLocation).Date()
	bY, bM, bD := b.In(ServerLocation).Date()
	aDate := time.Date(aY, aM, aD, 0, 0, 0, 0, ServerLocation)
	bDate := time.Date(bY, bM, bD, 0, 0, 0, 0, ServerLocation)
	days := aDate.Sub(bDate) / (time.Hour * 24)
	if days < 0 {
		return int(-days)
	}
	return int(days)
}

// GetWeekStart 返回 t 所在自然周的周一 0 点
// 自然周定义为: 周一 00:00:00 ~ 周日 23:59:59
// 注意: Go 的 time.Weekday 枚举中 Sunday=0, Monday=1, ..., Saturday=6
// 因为 Sunday 在枚举值上比 Monday 小,直接相减会得到 -1,
// 而 Sunday 实际上属于本周最后一天,需要回退 6 天到本周周一
func GetWeekStart(t time.Time) time.Time {
	t = t.In(ServerLocation)
	daysSinceMonday := int(t.Weekday() - time.Monday)
	if daysSinceMonday < 0 { // 即 Sunday, 需特殊处理
		daysSinceMonday = 6
	}
	y, m, d := t.Date()
	return time.Date(y, m, d-daysSinceMonday, 0, 0, 0, 0, ServerLocation)
}

// GetMonthStart 返回 t 所在自然月的 1 日 0 点
func GetMonthStart(t time.Time) time.Time {
	t = t.In(ServerLocation)
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, ServerLocation)
}

// IsSameWeek 判断两个时刻是否处于同一自然周
// 通过比较各自所在周的周一 0 点是否相同来实现
func IsSameWeek(a, b time.Time) bool {
	return GetWeekStart(a).Equal(GetWeekStart(b))
}

// IsSameMonth 判断两个时刻是否处于同一自然月
// 必须同时比较年份和月份,避免跨年但月份相同的情况(如 2025-12 与 2026-12)被误判
func IsSameMonth(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month()
}
