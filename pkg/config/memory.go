package config

import "time"

// MemoryClientConfig 记忆客户端配置（从 MemoryServiceConfig 派生）
type MemoryClientConfig struct {
	BaseURL       string
	Timeout       time.Duration
	MaxRetries    int
	RetryInterval time.Duration
}

// MemoryServiceConfig 记忆服务配置
type MemoryServiceConfig struct {
	Enabled bool   `mapstructure:"memory_enabled"`
	BaseURL string `mapstructure:"memory_service_url"`
	Dir     string `mapstructure:"memory_dir"`

	// 客户端配置
	Timeout       time.Duration `mapstructure:"memory_timeout"`
	MaxRetries    int           `mapstructure:"memory_max_retries"`
	RetryInterval time.Duration `mapstructure:"memory_retry_interval"`

	// 监控配置
	MonitorPollInterval int           `mapstructure:"memory_monitor_poll_interval"`
	MonitorMaxRetries   int           `mapstructure:"memory_monitor_max_retries"`
	MonitorTimeout      time.Duration `mapstructure:"memory_monitor_timeout"`

	// 触发阈值
	TriggerThreshold float64 `mapstructure:"memory_trigger_threshold"`
}

// ToClientConfig 转换为客户端配置
func (c *MemoryServiceConfig) ToClientConfig() *MemoryClientConfig {
	return &MemoryClientConfig{
		BaseURL:       c.BaseURL,
		Timeout:       c.Timeout,
		MaxRetries:    c.MaxRetries,
		RetryInterval: c.RetryInterval,
	}
}

// DefaultMemoryClientConfig 创建默认客户端配置
func DefaultMemoryClientConfig() *MemoryClientConfig {
	return &MemoryClientConfig{
		BaseURL:       DefaultMemoryServiceURL,
		Timeout:       DefaultMemoryServiceTimeout * time.Second,
		MaxRetries:    DefaultMemoryMaxRetries,
		RetryInterval: DefaultMemoryRetryInterval * time.Second,
	}
}
