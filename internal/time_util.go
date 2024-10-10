package internal

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
