package logger

import (
	"github.com/fish-tennis/gnet"
	"runtime"
)

var _logger = gnet.NewStdLogger(3)

func GetLogger() gnet.Logger {
	return _logger
}

func SetLogger(logger gnet.Logger) {
	_logger = logger
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
