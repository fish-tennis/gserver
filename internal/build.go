package internal

import (
	"runtime/debug"
	"time"
)

// 编译时注入变量 -X 只能设置string
var (
	BuildTime  = ""   // 编译时间
	GitVersion string // 对应的git版本
	BuildType  string // 构建方式: docker构建时传入docker
)

// init 兜底填充GitVersion,取值优先级: ldflags注入 > 构建时VCS嵌入 > 空串
// 为什么放在init: main的启动日志、告警模块(InitAlert)、NewBaseServer组装serverInfo等
// 所有使用点都在包初始化完成后才执行,在init里确定最终值可保证它们拿到的都是同一个结果
func init() {
	if GitVersion != "" {
		// ldflags(-X)注入的值优先级最高,不覆盖
		return
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	// go build在git仓库内默认(-buildvcs=auto)会把vcs.revision/vcs.time嵌入二进制;
	// go run/测试二进制/docker等无git上下文的场景取不到这些设置,保持空串容错
	var revision, commitDate string
	for _, setting := range bi.Settings {
		switch setting.Key {
		case "vcs.revision":
			// 截短为7位短hash,与git短hash展示习惯一致,便于人工比对
			if len(setting.Value) > 7 {
				revision = setting.Value[:7]
			} else {
				revision = setting.Value
			}
		case "vcs.time":
			// 格式形如2026-09-02T15:04:05Z(RFC3339),只取日期部分,便于阅读提交时间
			if t, err := time.Parse(time.RFC3339, setting.Value); err == nil {
				commitDate = t.Format("2006-01-02")
			}
		}
	}
	if revision == "" {
		return
	}
	if commitDate != "" {
		GitVersion = revision + "(" + commitDate + ")"
	} else {
		GitVersion = revision
	}
}
