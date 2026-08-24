package util

import "math"

// 乘法结果是否数值溢出
func IsMultiOverflow(a, b int32) bool {
	result := int64(a) * int64(b)
	return result > math.MaxInt32 || result < math.MinInt32
}

// AddInt32 int32 安全加法,溢出时钳制到 int32 的上下边界
// 避免每个调用点都编写 if-else 来判断溢出后再赋值
func AddInt32(a, b int32) int32 {
	r := int64(a) + int64(b)
	if r > math.MaxInt32 {
		return math.MaxInt32
	}
	if r < math.MinInt32 {
		return math.MinInt32
	}
	return int32(r)
}

// MulInt32 int32 安全乘法,溢出时钳制到 int32 的上下边界
// 避免每个调用点都编写 if-else 来判断溢出后再赋值
func MulInt32(a, b int32) int32 {
	r := int64(a) * int64(b)
	if r > math.MaxInt32 {
		return math.MaxInt32
	}
	if r < math.MinInt32 {
		return math.MinInt32
	}
	return int32(r)
}

// AddInt64 int64 安全加法,溢出时钳制到 int64 的上下边界
// int64 无法用更高精度中转,这里用符号位变化规律检测溢出:
// 正数+正数结果变负 => 上溢;负数+负数结果变正 => 下溢
func AddInt64(a, b int64) int64 {
	r := a + b
	// 正数加上导致结果变小,说明上溢
	if b > 0 && r < a {
		return math.MaxInt64
	}
	// 负数相加导致结果变大,说明下溢
	if b < 0 && r > a {
		return math.MinInt64
	}
	return r
}

// MulInt64 int64 安全乘法,溢出时钳制到 int64 的上下边界
// 用除法回验检测溢出:若 r/a != b(且 a!=0),说明结果已被截断,发生了溢出
// 另需单独处理 a 或 b 为 -1 且另一个为 MinInt64 的特殊情况(该乘法本身会因补码回环得到错误结果)
func MulInt64(a, b int64) int64 {
	if a == 0 || b == 0 {
		return 0
	}
	// MinInt64 * -1 会因补码回环得到 MinInt64,属于溢出,直接钳制
	if a == math.MinInt64 && b == -1 {
		return math.MaxInt64
	}
	if b == math.MinInt64 && a == -1 {
		return math.MaxInt64
	}
	r := a * b
	// 除法回验:正确结果应有 r/a == b
	if r/a != b {
		// 同号应得正,同号却溢出 => 上溢;异号溢出 => 下溢
		if (a > 0) == (b > 0) {
			return math.MaxInt64
		}
		return math.MinInt64
	}
	return r
}
