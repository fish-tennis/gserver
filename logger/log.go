package logger

import (
	"log/slog"
	"runtime"
)

// LogStack 打印当前 goroutine 的堆栈信息
func LogStack() {
	buf := make([]byte, 1<<12)
	slog.Error(string(buf[:runtime.Stack(buf, false)]))
}
