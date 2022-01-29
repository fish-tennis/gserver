package util

import (
	"github.com/fish-tennis/snowflake"
)

// 雪花算法
var snowFlake *snowflake.SnowFlake

func InitIdGenerator(workerId uint16) {
	snowFlake = snowflake.NewSnowFlake(workerId)
}

// 生成唯一id
func GenUniqueId() int64 {
	return int64(snowFlake.NextId())
}
