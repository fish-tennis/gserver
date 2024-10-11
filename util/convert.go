package util

import (
	"strconv"
	"strings"
)

func ToInt(v any) int {
	switch i := v.(type) {
	case int:
		return i
	case int8:
		return int(i)
	case int16:
		return int(i)
	case int32:
		return int(i)
	case int64:
		return int(i)
	case uint:
		return int(i)
	case uint8:
		return int(i)
	case uint16:
		return int(i)
	case uint32:
		return int(i)
	case uint64:
		return int(i)
	case float32:
		return int(i)
	case float64:
		return int(i)
	case string:
		i64, err := strconv.ParseInt(i, 10, 64)
		if err != nil {
			return 0
		}
		return int(i64)
	}
	return 0
}

func ToUint(v any) uint {
	switch i := v.(type) {
	case int:
		return uint(i)
	case int8:
		return uint(i)
	case int16:
		return uint(i)
	case int32:
		return uint(i)
	case int64:
		return uint(i)
	case uint:
		return i
	case uint8:
		return uint(i)
	case uint16:
		return uint(i)
	case uint32:
		return uint(i)
	case uint64:
		return uint(i)
	case float32:
		return uint(i)
	case float64:
		return uint(i)
	case string:
		u64, err := strconv.ParseUint(i, 10, 64)
		if err != nil {
			return 0
		}
		return uint(u64)
	}
	return 0
}

func ToFloat(v any) float64 {
	switch i := v.(type) {
	case int:
		return float64(i)
	case int8:
		return float64(i)
	case int16:
		return float64(i)
	case int32:
		return float64(i)
	case int64:
		return float64(i)
	case uint:
		return float64(i)
	case uint8:
		return float64(i)
	case uint16:
		return float64(i)
	case uint32:
		return float64(i)
	case uint64:
		return float64(i)
	case float32:
		return float64(i)
	case float64:
		return i
	case string:
		f64, err := strconv.ParseFloat(i, 64)
		if err != nil {
			return 0
		}
		return f64
	}
	return 0
}

func ToBool(v any) bool {
	switch i := v.(type) {
	case bool:
		return i
	case string:
		return strings.ToLower(i) == "true" || v == "1"
	}
	return false
}
