package util

import "time"

// 获取当前时间戳(秒)
func GetCurrentTimeStamp() uint32 {
	return uint32(time.Now().UnixNano()/int64(time.Second))
}

// 获取当前毫秒数(毫秒,0.001秒)
func GetCurrentMS() int64 {
	return time.Now().UnixNano()/int64(time.Millisecond)
}