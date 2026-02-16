package logger

import (
	"os"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// Log 全局日志实例
	Log *zap.Logger
)

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

// Init 初始化日志系统
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

	// 创建 Logger
	Log = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

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

// Sync 同步日志缓冲区
func Sync() {
	if Log != nil {
		_ = Log.Sync()
	}
}

// getEnv 获取环境变量，提供默认值
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
