package util

import (
	"math"
	"time"

	"github.com/fish-tennis/gserver/pb"
)

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
		return int32(time.Date(int(y), time.Month(int(m)), int(d), 0, 0, 0, 0, time.Local).Unix())
	}
	return 0
}

// 去除Time中的时分秒,只保留日期
func ToDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

// 转换成20240219格式
func ToDateInt(t time.Time) int32 {
	y, m, d := t.Date()
	return int32(y*10000 + int(m)*100 + d)
}

// 2个日期的相隔天数
func DayCount(a time.Time, b time.Time) int {
	y, m, d := a.Date()
	bY, bM, bD := b.Date()
	aDate := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	bDate := time.Date(bY, bM, bD, 0, 0, 0, 0, time.Local)
	days := aDate.Sub(bDate) / (time.Hour * 24)
	if days < 0 {
		return int(-days)
	}
	return int(days)
}

// GetWeekStart 返回 t 所在自然周的周一 0 点
func GetWeekStart(t time.Time) time.Time {
	daysSinceMonday := int(t.Weekday() - time.Monday)
	if daysSinceMonday < 0 { // 即 Sunday, 需特殊处理
		daysSinceMonday = 6
	}
	y, m, d := t.Date()
	return time.Date(y, m, d-daysSinceMonday, 0, 0, 0, 0, time.Local)
}

// GetMonthStart 返回 t 所在自然月的 1 日 0 点
func GetMonthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.Local)
}

// IsSameWeek 判断两个时刻是否处于同一自然周
func IsSameWeek(a, b time.Time) bool {
	return GetWeekStart(a).Equal(GetWeekStart(b))
}

// IsSameMonth 判断两个时刻是否处于同一自然月
func IsSameMonth(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month()
}
