package util

import (
	"github.com/fish-tennis/gserver/logger"
	"strconv"
)

func Atoi(s string) int {
	i,err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}

func Atoi64(s string) int64 {
	i,err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return i
}

func Atou(s string) uint64 {
	u,err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return u
}

func Itoa(i interface{}) string {
	switch v := i.(type) {
	case int:
		return strconv.FormatInt(int64(v), 10)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', 2, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', 2, 64)
	}
	logger.Error("Itoa not support type:%v", i)
	return ""
}