package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
	"github.com/yourname/oxencode/pkg/logger"
	"github.com/yourname/oxencode/pkg/paths"
)

var cfg *Config

// Load 加载配置
func Load() (*Config, error) {
	log := logger.New("config")
	log.Info("Loading configuration")

	v := viper.New()

	// 初始化路径模块
	if err := paths.Init(); err != nil {
		log.Error("Failed to initialize paths", "error", err)
		return nil, err
	}

	configPath := paths.ConfigFile()

	log.Debug("Config path", "path", configPath)

	// 设置配置文件选项
	v.SetConfigFile(configPath)
	v.SetConfigType("toml")

	// 设置环境变量前缀
	v.SetEnvPrefix("OXENCODE")
	v.AutomaticEnv()

	// 绑定多种 API Key 环境变量
	v.BindEnv("api_key", "ANTHROPIC_API_KEY")
	v.BindEnv("api_key", "OPENAI_API_KEY")
	v.BindEnv("api_key", "AZURE_OPENAI_API_KEY")
	v.BindEnv("api_key", "GOOGLE_API_KEY")

	// 应用默认值
	applyDefaults(v)

	// 尝试读取配置文件
	configExists := true
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configExists = false
		log.Debug("User config not found", "path", configPath)
	}

	if configExists {
		log.Info("Loading user config", "path", configPath)
		if err := v.ReadInConfig(); err != nil {
			log.Error("Failed to read config", "error", err)
			return nil, err
		}
	} else {
		// 开发阶段：尝试从项目目录加载示例配置
		workDir, err := os.Getwd()
		if err == nil {
			exampleConfigPath := filepath.Join(workDir, "config.example.toml")
			if _, err := os.Stat(exampleConfigPath); err == nil {
				log.Info("Loading example config (development mode)", "path", exampleConfigPath)
				v.SetConfigFile(exampleConfigPath)
				if err := v.ReadInConfig(); err != nil {
					log.Error("Failed to read example config", "error", err)
					return nil, err
				}
			}
		}
	}

	// 解析配置
	cfg = &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		log.Error("Failed to unmarshal config", "error", err)
		return nil, err
	}

	// 计算派生值
	computeDerivedValues(cfg)

	// 处理工作目录：转换为绝对路径
	if cfg.WorkDir != "" {
		absPath, err := filepath.Abs(cfg.WorkDir)
		if err != nil {
			log.Error("Failed to resolve work directory to absolute path", "path", cfg.WorkDir, "error", err)
			return nil, fmt.Errorf("failed to resolve work directory: %w", err)
		}
		cfg.WorkDir = absPath
		log.Debug("Work directory resolved", "path", cfg.WorkDir)
	} else {
		// 默认使用当前目录
		cwd, err := os.Getwd()
		if err != nil {
			log.Warn("Failed to get current directory", "error", err)
		} else {
			cfg.WorkDir = cwd
			log.Debug("Using current directory as work directory", "path", cwd)
		}
	}

	log.Info("Configuration loaded",
		"provider", cfg.LLM.Provider,
		"model", cfg.LLM.Model,
		"max_tokens", cfg.LLM.MaxTokens,
		"temperature", cfg.LLM.Temperature,
		"work_dir", cfg.WorkDir,
	)

	// 验证配置
	if err := validateConfig(cfg); err != nil {
		log.Error("Config validation failed", "error", err)
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// applyDefaults 应用所有默认值
func applyDefaults(v *viper.Viper) {
	// LLM 配置
	v.SetDefault("provider", DefaultProvider)
	v.SetDefault("model", DefaultModel)
	v.SetDefault("max_tokens", DefaultMaxTokens)
	v.SetDefault("temperature", DefaultTemperature)
	v.SetDefault("thinking_budget", DefaultThinkingBudget)
	v.SetDefault("thinking_enabled", DefaultThinkingEnabled)

	// 上下文配置
	v.SetDefault("max_context_tokens", DefaultMaxContextTokens)
	v.SetDefault("max_page_tokens", DefaultMaxPageTokens)
	v.SetDefault("hard_max_l0_percent", DefaultHardMaxL0Percent)
	v.SetDefault("hard_max_l1_percent", DefaultHardMaxL1Percent)
	v.SetDefault("soft_max_l1_ratio", DefaultSoftMaxL1Ratio)
	v.SetDefault("hard_max_l2_percent", DefaultHardMaxL2Percent)
	v.SetDefault("soft_max_l2_ratio", DefaultSoftMaxL2Ratio)
	v.SetDefault("l1_max_tool_output_length", DefaultL1MaxToolLength)
	v.SetDefault("l1_max_assistant_length", DefaultL1MaxAssistantLen)
	v.SetDefault("compress_timeout", DefaultL0CompressTimeout)

	// 工具配置
	v.SetDefault("tool_timeout", DefaultToolTimeout)
	v.SetDefault("tool_max_retries", DefaultToolMaxRetries)

	// ReAct 循环配置
	v.SetDefault("max_react_iterations", DefaultMaxReActIterations)
	v.SetDefault("tool_output_max_length", DefaultToolOutputMaxLength)

	// 压缩配置
	v.SetDefault("compressor_model", DefaultCompressorModel)
	v.SetDefault("compressor_max_tokens", DefaultCompressorMaxTokens)
	v.SetDefault("compressor_temperature", DefaultCompressorTemperature)
	v.SetDefault("compressor_max_retries", DefaultCompressorMaxRetries)
	v.SetDefault("compress_worker_count", DefaultCompressWorkerCount)
	v.SetDefault("compress_queue_size", DefaultCompressQueueSize)

	// 记忆服务配置
	v.SetDefault("memory_service_url", DefaultMemoryServiceURL)
	v.SetDefault("memory_enabled", true)
	v.SetDefault("memory_dir", paths.MemoryDir())
	v.SetDefault("memory_monitor_poll_interval", DefaultMemoryMonitorPoll)
	v.SetDefault("memory_monitor_max_retries", DefaultMemoryMonitorRetries)
	v.SetDefault("memory_monitor_timeout", DefaultMemoryMonitorTimeout)
	v.SetDefault("memory_trigger_threshold", DefaultMemoryTriggerThreshold)

	// 路径配置
	v.SetDefault("work_dir", DefaultWorkDir)
	v.SetDefault("prompt_dir", DefaultPromptDir)
	v.SetDefault("archive_dir", DefaultArchiveDir)
}

// computeDerivedValues 计算派生值
func computeDerivedValues(c *Config) {
	// 计算时间类型字段
	if c.Memory.Timeout == 0 {
		c.Memory.Timeout = time.Duration(DefaultMemoryServiceTimeout) * time.Second
	}

	if c.Memory.RetryInterval == 0 {
		c.Memory.RetryInterval = time.Duration(DefaultMemoryRetryInterval) * time.Second
	}

	if c.Memory.MonitorTimeout == 0 {
		c.Memory.MonitorTimeout = time.Duration(DefaultMemoryMonitorTimeout) * time.Second
	}

	// 计算压缩工作器超时
	if c.CompressWorker.Timeout == 0 {
		c.CompressWorker.Timeout = time.Duration(c.Context.CompressTimeout) * time.Second
	}

	// 设置 Provider Base URL（如果未配置）
	if c.LLM.BaseURL == "" {
		c.LLM.BaseURL = ProviderBaseURLs[string(c.LLM.Provider)]
	}

	// 初始化 UI 颜色
	if c.UI.Colors == nil {
		c.UI.Colors = &UIColorConfig{}
	}

	// 设置默认 MaxRetries
	if c.Tool.MaxRetries == 0 {
		c.Tool.MaxRetries = DefaultToolMaxRetries
	}

	if c.Memory.MaxRetries == 0 {
		c.Memory.MaxRetries = DefaultMemoryMaxRetries
	}

	if c.Compressor.MaxRetries == 0 {
		c.Compressor.MaxRetries = DefaultCompressorMaxRetries
	}

	if c.ReAct.MaxIterations == 0 {
		c.ReAct.MaxIterations = DefaultMaxReActIterations
	}
}

// validateConfig 验证配置
func validateConfig(c *Config) error {
	// 验证 provider
	switch c.LLM.Provider {
	case ProviderAnthropic, ProviderOpenAI, ProviderAzure, ProviderBedrock,
		ProviderGoogle, ProviderOpenRouter, ProviderVercel, ProviderOpenAICompat,
		ProviderQwen, ProviderDeepSeek, ProviderGLM:
		// 有效的 provider
	default:
		return fmt.Errorf("unknown provider: %s", c.LLM.Provider)
	}

	// 验证 provider 特定配置
	if c.LLM.Azure != nil {
		if c.LLM.Azure.Endpoint == "" {
			return fmt.Errorf("azure provider requires azure_endpoint")
		}
	}

	if c.LLM.Google != nil {
		if c.LLM.Google.Location != "" && c.LLM.Google.Project == "" {
			return fmt.Errorf("google vertex ai requires google_project when google_location is set")
		}
	}

	return nil
}

// Get 获取配置实例
func Get() *Config {
	return cfg
}
