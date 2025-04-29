package util

import (
	"github.com/fish-tennis/gserver/pb"
	"time"
)

// 计算超时时间戳
func GetTimeoutTimestamp(timeType, timeout int32) int32 {
	switch timeType {
	case int32(pb.TimeType_TimeType_Timestamp):
		return timeout
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
