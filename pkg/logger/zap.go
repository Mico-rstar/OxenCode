package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// zapLogger Logger 接口的 zap 实现
type zapLogger struct {
	*zap.SugaredLogger
	name string
}

// Init 使用 zap 初始化日志系统
func Init(cfg *Config) error {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// 解析日志级别
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}

	// 编码器配置
	var encoderConfig zapcore.EncoderConfig
	if cfg.DevMode {
		// 开发模式：使用友好的格式
		encoderConfig = zap.NewDevelopmentEncoderConfig()
	} else {
		// 生产模式：使用 JSON 格式
		encoderConfig = zap.NewProductionEncoderConfig()
	}
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// 输出配置
	var writer zapcore.WriteSyncer
	if cfg.OutputPath != "" {
		file, err := os.OpenFile(cfg.OutputPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		writer = zapcore.AddSync(file)
	} else {
		writer = zapcore.AddSync(os.Stdout)
	}

	// 创建 Core
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		writer,
		level,
	)

	// 创建 zap logger
	baseLogger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	// 包装为 zapLogger 并设置为全局
	global = &zapLogger{
		SugaredLogger: baseLogger.Sugar(),
		name:          "root",
	}

	return nil
}

// InitFromEnv 从环境变量初始化
func InitFromEnv() error {
	cfg := &Config{
		Level:      getEnv("LOG_LEVEL", "info"),
		DevMode:    getEnv("LOG_DEV", "true") == "true",
		OutputPath: getEnv("LOG_FILE", "debug.log"),
	}
	return Init(cfg)
}

// NewWithLogger 使用指定的 zap logger 创建命名 logger
// 支持依赖注入，便于测试
func NewWithLogger(name string, base *zap.Logger) Logger {
	if base == nil {
		return &noopLogger{}
	}
	return &zapLogger{
		SugaredLogger: base.Sugar().Named(name),
		name:          name,
	}
}

// Debug 记录调试级别日志
func (l *zapLogger) Debug(msg string, args ...interface{}) {
	l.SugaredLogger.Debugw(msg, args...)
}

// Info 记录信息级别日志
func (l *zapLogger) Info(msg string, args ...interface{}) {
	l.SugaredLogger.Infow(msg, args...)
}

// Warn 记录警告级别日志
func (l *zapLogger) Warn(msg string, args ...interface{}) {
	l.SugaredLogger.Warnw(msg, args...)
}

// Error 记录错误级别日志
func (l *zapLogger) Error(msg string, args ...interface{}) {
	l.SugaredLogger.Errorw(msg, args...)
}

// Fatal 记录致命错误日志后退出程序
func (l *zapLogger) Fatal(msg string, args ...interface{}) {
	l.SugaredLogger.Fatalw(msg, args...)
}

// Named 创建子 logger
func (l *zapLogger) Named(name string) Logger {
	return &zapLogger{
		SugaredLogger: l.SugaredLogger.Named(name),
		name:          l.name + "." + name,
	}
}

// With 创建带字段的 logger
func (l *zapLogger) With(args ...interface{}) Logger {
	return &zapLogger{
		SugaredLogger: l.SugaredLogger.With(args...),
		name:          l.name,
	}
}

// Sync 同步日志缓冲区
func (l *zapLogger) Sync() error {
	return l.SugaredLogger.Sync()
}

// getEnv 获取环境变量，提供默认值
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
