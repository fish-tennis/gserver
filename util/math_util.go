package util

import "math"

// 乘法结果是否数值溢出
func IsMultiOverflow(a, b int32) bool {
	if int64(a)*int64(b) > math.MaxInt32 {
		return true
	}
	return false
}
