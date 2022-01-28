package util

import "time"

// 雪花算法
type SnowFlake struct {

}

// 生成唯一id
func GenUniqueId() int64 {
	// TODO:雪花算法实现
	return time.Now().UnixNano()
}
