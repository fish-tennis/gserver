package util

import "math"

// 乘法结果是否数值溢出
func IsMultiOverflow(a, b int32) bool {
	result := int64(a) * int64(b)
	return result > math.MaxInt32 || result < math.MinInt32
}
