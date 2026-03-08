package logger

// Logger 统一的日志记录器接口
// 定义了项目所需的核心日志方法
type Logger interface {
	// Debug 记录调试级别日志
	Debug(msg string, args ...interface{})

	// Info 记录信息级别日志
	Info(msg string, args ...interface{})

	// Warn 记录警告级别日志
	Warn(msg string, args ...interface{})

	// Error 记录错误级别日志
	Error(msg string, args ...interface{})

	// Fatal 记录致命错误日志后退出程序
	Fatal(msg string, args ...interface{})

	// Named 创建子 logger（用于命名空间）
	Named(name string) Logger

	// With 创建带字段的 logger
	With(args ...interface{}) Logger

	// Sync 同步日志缓冲区
	Sync() error
}

// Config 日志配置
type Config struct {
	Level      string // debug, info, warn, error
	DevMode    bool   // 开发模式，使用更友好的格式
	OutputPath string // 输出路径，空则输出到 stdout
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Level:      "info",
		DevMode:    true,
		OutputPath: "debug.log",
	}
}

// global logger instance
var global Logger

// SetGlobal 设置全局 logger
func SetGlobal(l Logger) {
	global = l
}

// GetGlobal 获取全局 logger
func GetGlobal() Logger {
	return global
}

// New 创建命名 logger（使用全局 logger）
func New(name string) Logger {
	if global == nil {
		return &noopLogger{}
	}
	return global.Named(name)
}

// === 全局便捷函数 ===

// Debug 记录调试级别日志（使用全局 logger）
func Debug(msg string, args ...interface{}) {
	if global != nil {
		global.Debug(msg, args...)
	}
}

// Info 记录信息级别日志（使用全局 logger）
func Info(msg string, args ...interface{}) {
	if global != nil {
		global.Info(msg, args...)
	}
}

// Warn 记录警告级别日志（使用全局 logger）
func Warn(msg string, args ...interface{}) {
	if global != nil {
		global.Warn(msg, args...)
	}
}

// Error 记录错误级别日志（使用全局 logger）
func Error(msg string, args ...interface{}) {
	if global != nil {
		global.Error(msg, args...)
	}
}

// Fatal 记录致命错误日志后退出程序（使用全局 logger）
func Fatal(msg string, args ...interface{}) {
	if global != nil {
		global.Fatal(msg, args...)
	}
}

// Sync 同步全局日志缓冲区
func Sync() {
	if global != nil {
		_ = global.Sync()
	}
}

// noopLogger no-op logger 实现，用于未初始化的情况
type noopLogger struct{}

func (n *noopLogger) Debug(msg string, args ...interface{})                                   {}
func (n *noopLogger) Info(msg string, args ...interface{})                                    {}
func (n *noopLogger) Warn(msg string, args ...interface{})                                    {}
func (n *noopLogger) Error(msg string, args ...interface{})                                   {}
func (n *noopLogger) Fatal(msg string, args ...interface{})                                   {}
func (n *noopLogger) Named(name string) Logger                                                { return n }
func (n *noopLogger) With(args ...interface{}) Logger                                         { return n }
func (n *noopLogger) Sync() error                                                              { return nil }

// NewNop 创建一个 no-op logger，用于测试或不需要日志的场景
func NewNop() Logger {
	return &noopLogger{}
}
