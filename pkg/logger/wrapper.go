package logger

import (
	"go.uber.org/zap"
)

// Logger 提供简洁的日志 API，封装 zap.SugaredLogger
type Logger struct {
	*zap.SugaredLogger
	name string
}

// New 创建命名日志记录器
// 如果全局 Log 未初始化，返回 no-op logger
func New(name string) *Logger {
	base := Log
	if base == nil {
		base = zap.NewNop()
	}
	return &Logger{
		SugaredLogger: base.Sugar().Named(name),
		name:          name,
	}
}

// NewWithLogger 使用指定的 logger 创建命名日志记录器
// 支持依赖注入，便于测试
func NewWithLogger(name string, base *zap.Logger) *Logger {
	if base == nil {
		base = zap.NewNop()
	}
	return &Logger{
		SugaredLogger: base.Sugar().Named(name),
		name:          name,
	}
}

// Named 创建子 logger
func (l *Logger) Named(name string) *Logger {
	return &Logger{
		SugaredLogger: l.SugaredLogger.Named(name),
		name:          l.name + "." + name,
	}
}

// With 创建带字段的日志记录器
func (l *Logger) With(args ...interface{}) *Logger {
	return &Logger{
		SugaredLogger: l.SugaredLogger.With(args...),
		name:          l.name,
	}
}

// Debug 记录调试级别日志
func (l *Logger) Debug(msg string, args ...interface{}) {
	l.SugaredLogger.Debugw(msg, args...)
}

// Info 记录信息级别日志
func (l *Logger) Info(msg string, args ...interface{}) {
	l.SugaredLogger.Infow(msg, args...)
}

// Warn 记录警告级别日志
func (l *Logger) Warn(msg string, args ...interface{}) {
	l.SugaredLogger.Warnw(msg, args...)
}

// Error 记录错误级别日志
func (l *Logger) Error(msg string, args ...interface{}) {
	l.SugaredLogger.Errorw(msg, args...)
}

// Fatal 记录致命错误日志后退出程序
func (l *Logger) Fatal(msg string, args ...interface{}) {
	l.SugaredLogger.Fatalw(msg, args...)
}

// DPkg 为 Printf 风格的日志记录提供支持（别名）
func (l *Logger) DPkg(msg string, args ...interface{}) {
	l.SugaredLogger.Debugw(msg, args...)
}

// IPkg 为 Printf 风格的日志记录提供支持（别名）
func (l *Logger) IPkg(msg string, args ...interface{}) {
	l.SugaredLogger.Infow(msg, args...)
}

// WPkg 为 Printf 风格的日志记录提供支持（别名）
func (l *Logger) WPkg(msg string, args ...interface{}) {
	l.SugaredLogger.Warnw(msg, args...)
}

// EPkg 为 Printf 风格的日志记录提供支持（别名）
func (l *Logger) EPkg(msg string, args ...interface{}) {
	l.SugaredLogger.Errorw(msg, args...)
}

// === 全局便捷函数 ===

// Debug 记录调试级别日志（使用全局 logger）
func Debug(msg string, args ...interface{}) {
	if Log != nil {
		Log.Sugar().Debugw(msg, args...)
	}
}

// Info 记录信息级别日志（使用全局 logger）
func Info(msg string, args ...interface{}) {
	if Log != nil {
		Log.Sugar().Infow(msg, args...)
	}
}

// Warn 记录警告级别日志（使用全局 logger）
func Warn(msg string, args ...interface{}) {
	if Log != nil {
		Log.Sugar().Warnw(msg, args...)
	}
}

// Error 记录错误级别日志（使用全局 logger）
func Error(msg string, args ...interface{}) {
	if Log != nil {
		Log.Sugar().Errorw(msg, args...)
	}
}

// Fatal 记录致命错误日志后退出程序（使用全局 logger）
func Fatal(msg string, args ...interface{}) {
	if Log != nil {
		Log.Sugar().Fatalw(msg, args...)
	}
}
