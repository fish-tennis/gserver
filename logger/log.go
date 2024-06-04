package logger

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
)

func Debug(format string, args ...interface{}) {
	if !slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		return
	}
	if len(args) == 0 {
		slog.Debug(format)
		return
	}
	slog.Debug(fmt.Sprintf(format, args))
}

func Info(format string, args ...interface{}) {
	if !slog.Default().Enabled(context.Background(), slog.LevelInfo) {
		return
	}
	if len(args) == 0 {
		slog.Info(format)
		return
	}
	slog.Info(fmt.Sprintf(format, args))
}

func Warn(format string, args ...interface{}) {
	if !slog.Default().Enabled(context.Background(), slog.LevelWarn) {
		return
	}
	if len(args) == 0 {
		slog.Warn(format)
		return
	}
	slog.Warn(fmt.Sprintf(format, args))
}

func Error(format string, args ...interface{}) {
	if !slog.Default().Enabled(context.Background(), slog.LevelError) {
		return
	}
	if len(args) == 0 {
		slog.Error(format)
		return
	}
	slog.Error(fmt.Sprintf(format, args))
}

func LogStack() {
	buf := make([]byte, 1<<12)
	Error(string(buf[:runtime.Stack(buf, false)]))
}
