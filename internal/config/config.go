package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	APIKey     string `mapstructure:"api_key"`
	Model      string `mapstructure:"model"`
	MaxTokens  int    `mapstructure:"max_tokens"`
	Temperature float64 `mapstructure:"temperature"`
}

var cfg *Config

// Load 加载配置
func Load() (*Config, error) {
	v := viper.New()

	// 设置配置文件路径
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configDir := filepath.Join(homeDir, ".config", "oxencode")
	configPath := filepath.Join(configDir, "config.toml")

	// 设置配置文件选项
	v.SetConfigFile(configPath)
	v.SetConfigType("toml")

	// 设置环境变量
	v.SetEnvPrefix("OXENCODE")
	v.AutomaticEnv()

	// 设置默认值
	v.SetDefault("model", "claude-3-5-sonnet-20241022")
	v.SetDefault("max_tokens", 4096)
	v.SetDefault("temperature", 0.7)

	// 尝试读取配置文件（如果存在）
	if _, err := os.Stat(configPath); err == nil {
		if err := v.ReadInConfig(); err != nil {
			return nil, err
		}
	}

	// 解析配置
	cfg = &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Get 获取配置实例
func Get() *Config {
	return cfg
}
