package internal

// 编译时注入变量 -X 只能设置string
var (
	BuildTime  = ""   // 编译时间
	GitVersion string // 对应的git版本
	BuildType  string // 构建方式: docker构建时传入docker
)
