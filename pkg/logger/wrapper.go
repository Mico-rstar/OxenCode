package logger

import (
	"go.uber.org/zap"
)

// Logger 提供更简洁的日志 API
type Logger struct {
	*zap.SugaredLogger
	name string
}

// New 创建命名日志记录器
func New(name string) *Logger {
	return &Logger{
		SugaredLogger: Log.Sugar().Named(name),
		name:          name,
	}
}

// With 创建带字段的日志记录器
func (l *Logger) With(fields ...interface{}) *Logger {
	return &Logger{
		SugaredLogger: l.SugaredLogger.With(fields...),
		name:          l.name,
	}
}

// 便捷方法，直接使用全局日志
func Debug(msg string, fields ...interface{}) {
	Log.Sugar().Debugw(msg, fields...)
}

func Info(msg string, fields ...interface{}) {
	Log.Sugar().Infow(msg, fields...)
}

func Warn(msg string, fields ...interface{}) {
	Log.Sugar().Warnw(msg, fields...)
}

func Error(msg string, fields ...interface{}) {
	Log.Sugar().Errorw(msg, fields...)
}

func Fatal(msg string, fields ...interface{}) {
	Log.Sugar().Fatalw(msg, fields...)
}
