package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
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

	// 尝试读取用户配置文件
	configExists := true
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configExists = false
	}

	if configExists {
		if err := v.ReadInConfig(); err != nil {
			return nil, err
		}
	} else {
		// 开发阶段：尝试从项目目录加载示例配置
		workDir, err := os.Getwd()
		if err == nil {
			exampleConfigPath := filepath.Join(workDir, "config.example.toml")
			if _, err := os.Stat(exampleConfigPath); err == nil {
				v.SetConfigFile(exampleConfigPath)
				if err := v.ReadInConfig(); err != nil {
					return nil, err
				}
			}
		}
	}

	// 解析配置
	cfg = &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}

	// 验证配置
	if err := validateConfig(cfg); err != nil {
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
	if c.APIKey != "" {
		return c.APIKey
	}

	// 根据 provider 尝试不同的环境变量
	switch c.Provider {
	case ProviderAnthropic:
		if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
			return key
		}
	case ProviderOpenAI, ProviderOpenAICompat, ProviderOpenRouter, ProviderVercel:
		if key := os.Getenv("OPENAI_API_KEY"); key != "" {
			return key
		}
	case ProviderAzure:
		if key := os.Getenv("AZURE_OPENAI_API_KEY"); key != "" {
			return key
		}
	case ProviderGoogle:
		if key := os.Getenv("GOOGLE_API_KEY"); key != "" {
			return key
		}
	case ProviderQwen:
		if key := os.Getenv("DASHSCOPE_API_KEY"); key != "" {
			return key
		}
	case ProviderDeepSeek:
		if key := os.Getenv("DEEPSEEK_API_KEY"); key != "" {
			return key
		}
	case ProviderGLM:
		if key := os.Getenv("ZHIPU_API_KEY"); key != "" {
			return key
		}
	}

	return ""
}

// Get 获取配置实例
func Get() *Config {
	return cfg
}
