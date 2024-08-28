package logger

import (
	"context"
	"fmt"
	"github.com/fish-tennis/gnet"
	"log/slog"
	"runtime"
	"time"
)

var (
	_logger   = &slogWrapper{}
	_skipCall = 2
)

// see go\src\log\slog\example_wrap_test.go
type slogWrapper struct {
}

func (s *slogWrapper) Debug(format string, args ...interface{}) {
	if !slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(_skipCall, pcs[:]) // skip [Callers, Infof]
	r := slog.NewRecord(time.Now(), slog.LevelDebug, fmt.Sprintf(format, args...), pcs[0])
	_ = slog.Default().Handler().Handle(context.Background(), r)
}

func (s *slogWrapper) Info(format string, args ...interface{}) {
	if !slog.Default().Enabled(context.Background(), slog.LevelInfo) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(_skipCall, pcs[:]) // skip [Callers, Infof]
	r := slog.NewRecord(time.Now(), slog.LevelInfo, fmt.Sprintf(format, args...), pcs[0])
	_ = slog.Default().Handler().Handle(context.Background(), r)
}

func (s *slogWrapper) Warn(format string, args ...interface{}) {
	if !slog.Default().Enabled(context.Background(), slog.LevelWarn) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(_skipCall, pcs[:]) // skip [Callers, Infof]
	r := slog.NewRecord(time.Now(), slog.LevelWarn, fmt.Sprintf(format, args...), pcs[0])
	_ = slog.Default().Handler().Handle(context.Background(), r)
}

func (s *slogWrapper) Error(format string, args ...interface{}) {
	if !slog.Default().Enabled(context.Background(), slog.LevelError) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(_skipCall, pcs[:]) // skip [Callers, Infof]
	r := slog.NewRecord(time.Now(), slog.LevelError, fmt.Sprintf(format, args...), pcs[0])
	_ = slog.Default().Handler().Handle(context.Background(), r)
}

func GetLogger() gnet.Logger {
	return _logger
}

func Debug(format string, args ...interface{}) {
	_logger.Debug(format, args...)
}

func Info(format string, args ...interface{}) {
	_logger.Info(format, args...)
}

func Warn(format string, args ...interface{}) {
	_logger.Warn(format, args...)
}

func Error(format string, args ...interface{}) {
	_logger.Error(format, args...)
}

func LogStack() {
	buf := make([]byte, 1<<12)
	Error(string(buf[:runtime.Stack(buf, false)]))
}
