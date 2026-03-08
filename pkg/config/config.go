package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"github.com/yourname/oxencode/pkg/logger"
)

// ProviderType AI 服务提供商类型
type ProviderType string

const (
	ProviderAnthropic    ProviderType = "anthropic"
	ProviderOpenAI       ProviderType = "openai"
	ProviderAzure        ProviderType = "azure"
	ProviderBedrock      ProviderType = "bedrock"
	ProviderGoogle       ProviderType = "google"
	ProviderOpenRouter   ProviderType = "openrouter"
	ProviderVercel       ProviderType = "vercel"
	ProviderOpenAICompat ProviderType = "openaicompat"
	ProviderQwen         ProviderType = "qwen"
	ProviderDeepSeek     ProviderType = "deepseek"
	ProviderGLM          ProviderType = "glm"
)

// Config 应用配置
type Config struct {
	Provider    ProviderType `mapstructure:"provider"`
	APIKey      string       `mapstructure:"api_key"`
	Model       string       `mapstructure:"model"`
	MaxTokens   int          `mapstructure:"max_tokens"`
	Temperature float64      `mapstructure:"temperature"`

	// Azure 特定配置
	AzureEndpoint    string `mapstructure:"azure_endpoint"`
	AzureAPIVersion  string `mapstructure:"azure_api_version"`

	// Bedrock 特定配置
	BedrockRegion string `mapstructure:"bedrock_region"`

	// Google 特定配置
	GoogleProject  string `mapstructure:"google_project"`
	GoogleLocation string `mapstructure:"google_location"`

	// OpenAI 兼容 API 的 Base URL (Qwen, DeepSeek, GLM 等)
	BaseURL string `mapstructure:"base_url"`

	// 工作目录配置
	WorkDir string `mapstructure:"work_dir"` // 工作目录，默认为当前目录

	// 工具配置
	ToolTimeout int `mapstructure:"tool_timeout"` // 工具执行超时时间（秒），默认 120

	// Extended Thinking 配置
	// Anthropic Claude: token 预算（数字），0 表示不启用
	// OpenAI/OpenAICompat: reasoning_effort (字符串: minimal, low, medium, high)
	ThinkingBudget    int    `mapstructure:"thinking_budget"`    // Anthropic: token 预算
	ThinkingEffort    string `mapstructure:"thinking_effort"`    // OpenAI: reasoning effort level
	ThinkingEnabled   bool   `mapstructure:"thinking_enabled"`   // 通用开关，启用推理功能

	// 系统提示词配置
	PromptDir string `mapstructure:"prompt_dir"` // 系统提示词目录，默认为 pkg/prompt

	// Compressor 配置
	CompressorModel       string  `mapstructure:"compressor_model"`        // 用于上下文压缩的模型
	CompressorMaxTokens   int64     `mapstructure:"compressor_max_tokens"`   // 压缩时最大输出 token 数
	CompressorTemperature float64 `mapstructure:"compressor_temperature"`  // 压缩时温度参数
	CompressorMaxRetries  int     `mapstructure:"compressor_max_retries"`  // ReAct 循环最大重试次数

	// L1 预处理配置
	L1MaxToolOutputLength int `mapstructure:"l1_max_tool_output_length"` // 工具输出最大长度，0表示不截断
	L1MaxAssistantLength  int `mapstructure:"l1_max_assistant_length"`   // Assistant消息最大长度，0表示不截断

	// Page 兜底配置
	MaxPageTokens int `mapstructure:"max_page_tokens"` // 单Page最大token数，防止单任务上下文过长

	// ReAct 循环配置
	MaxReActIterations  int `mapstructure:"max_react_iterations"`   // ReAct 循环最大迭代次数
	ToolOutputMaxLength int `mapstructure:"tool_output_max_length"` // 工具输出最大长度，0表示使用默认值
}

var cfg *Config

// Load 加载配置
func Load() (*Config, error) {
	log := logger.New("config")
	log.Info("Loading configuration")

	v := viper.New()

	// 设置配置文件路径
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Error("Failed to get home directory", "error", err)
		return nil, err
	}

	configDir := filepath.Join(homeDir, ".config", "oxencode")
	configPath := filepath.Join(configDir, "config.toml")

	log.Debug("Config path", "path", configPath)

	// 设置配置文件选项
	v.SetConfigFile(configPath)
	v.SetConfigType("toml")

	// 设置环境变量
	v.SetEnvPrefix("OXENCODE")
	v.AutomaticEnv()

	// 绑定多种 API Key 环境变量
	v.BindEnv("api_key", "ANTHROPIC_API_KEY")
	v.BindEnv("api_key", "OPENAI_API_KEY")
	v.BindEnv("api_key", "AZURE_OPENAI_API_KEY")
	v.BindEnv("api_key", "GOOGLE_API_KEY")

	// 设置默认值
	v.SetDefault("provider", "anthropic")
	v.SetDefault("model", "claude-sonnet-4-5-20250514")
	v.SetDefault("max_tokens", 8192)
	v.SetDefault("temperature", 0.7)
	v.SetDefault("work_dir", ".")
	v.SetDefault("tool_timeout", 120)
	v.SetDefault("thinking_budget", 0)
	v.SetDefault("thinking_effort", "")
	v.SetDefault("thinking_enabled", false)
	v.SetDefault("prompt_dir", "pkg/prompt/prompts")

	// Compressor 默认配置
	v.SetDefault("compressor_model", "claude-sonnet-4-5-20250514")
	v.SetDefault("compressor_max_tokens", 4096)
	v.SetDefault("compressor_temperature", 0.3)
	v.SetDefault("compressor_max_retries", 3)

	// L1 预处理默认配置
	v.SetDefault("l1_max_tool_output_length", 1000)
	v.SetDefault("l1_max_assistant_length", 2000)

	// Page 兜底配置
	v.SetDefault("max_page_tokens", 50000)

	// 尝试读取用户配置文件
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
		"provider", cfg.Provider,
		"model", cfg.Model,
		"max_tokens", cfg.MaxTokens,
		"temperature", cfg.Temperature,
		"work_dir", cfg.WorkDir,
	)

	// 验证配置
	if err := validateConfig(cfg); err != nil {
		log.Error("Config validation failed", "error", err)
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// validateConfig 验证配置
func validateConfig(c *Config) error {
	// 验证 provider
	switch c.Provider {
	case ProviderAnthropic, ProviderOpenAI, ProviderAzure, ProviderBedrock,
		ProviderGoogle, ProviderOpenRouter, ProviderVercel, ProviderOpenAICompat,
		ProviderQwen, ProviderDeepSeek, ProviderGLM:
		// 有效的 provider
	default:
		return fmt.Errorf("unknown provider: %s", c.Provider)
	}

	// 验证 provider 特定配置
	switch c.Provider {
	case ProviderAzure:
		if c.AzureEndpoint == "" {
			return fmt.Errorf("azure provider requires azure_endpoint")
		}
	case ProviderGoogle:
		if c.GoogleLocation != "" && c.GoogleProject == "" {
			return fmt.Errorf("google vertex ai requires google_project when google_location is set")
		}
	}

	return nil
}

// GetAPIKeyFromEnv 从环境变量获取 API Key
func (c *Config) GetAPIKeyFromEnv() string {
	log := logger.New("config")

	if c.APIKey != "" {
		log.Debug("Using API key from config")
		return c.APIKey
	}

	// 根据 provider 尝试不同的环境变量
	log.Debug("Trying to get API key from environment", "provider", c.Provider)

	switch c.Provider {
	case ProviderAnthropic:
		if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
			log.Debug("Found ANTHROPIC_API_KEY")
			return key
		}
	case ProviderOpenAI, ProviderOpenAICompat, ProviderOpenRouter, ProviderVercel:
		if key := os.Getenv("OPENAI_API_KEY"); key != "" {
			log.Debug("Found OPENAI_API_KEY")
			return key
		}
	case ProviderAzure:
		if key := os.Getenv("AZURE_OPENAI_API_KEY"); key != "" {
			log.Debug("Found AZURE_OPENAI_API_KEY")
			return key
		}
	case ProviderGoogle:
		if key := os.Getenv("GOOGLE_API_KEY"); key != "" {
			log.Debug("Found GOOGLE_API_KEY")
			return key
		}
	case ProviderQwen:
		if key := os.Getenv("DASHSCOPE_API_KEY"); key != "" {
			log.Debug("Found DASHSCOPE_API_KEY")
			return key
		}
	case ProviderDeepSeek:
		if key := os.Getenv("DEEPSEEK_API_KEY"); key != "" {
			log.Debug("Found DEEPSEEK_API_KEY")
			return key
		}
	case ProviderGLM:
		if key := os.Getenv("ZHIPU_API_KEY"); key != "" {
			log.Debug("Found ZHIPU_API_KEY")
			return key
		}
	}

	log.Warn("No API key found in environment")
	return ""
}

// Get 获取配置实例
func Get() *Config {
	return cfg
}
