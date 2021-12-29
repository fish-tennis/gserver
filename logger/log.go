package logger

import (
	"github.com/fish-tennis/gnet"
)

var logger = gnet.NewStdLogger(3)

func Debug(format string, args ...interface{}) {
	logger.Debug(format, args...)
}

func Info(format string, args ...interface{}) {
	logger.Info(format, args...)
}

func Warn(format string, args ...interface{}) {
	logger.Warn(format, args...)
}

func Error(format string, args ...interface{}) {
	logger.Error(format, args...)
}