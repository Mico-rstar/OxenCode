package config

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

// AzureConfig Azure 特定配置
type AzureConfig struct {
	Endpoint   string `mapstructure:"azure_endpoint"`
	APIVersion string `mapstructure:"azure_api_version"`
}

// BedrockConfig Bedrock 特定配置
type BedrockConfig struct {
	Region string `mapstructure:"bedrock_region"`
}

// GoogleConfig Google 特定配置
type GoogleConfig struct {
	Project  string `mapstructure:"google_project"`
	Location string `mapstructure:"google_location"`
}

// LLMConfig LLM 配置
type LLMConfig struct {
	// 通用配置
	Provider    ProviderType `mapstructure:"provider"`
	APIKey      string       `mapstructure:"api_key"`
	Model       string       `mapstructure:"model"`
	MaxTokens   int          `mapstructure:"max_tokens"`
	Temperature float64      `mapstructure:"temperature"`

	// Provider 特定配置
	Azure   *AzureConfig   `mapstructure:"azure"`
	Bedrock *BedrockConfig `mapstructure:"bedrock"`
	Google  *GoogleConfig  `mapstructure:"google"`
	BaseURL string         `mapstructure:"base_url"`

	// Extended Thinking 配置
	// Anthropic Claude: token 预算（数字），0 表示不启用
	// OpenAI/OpenAICompat: reasoning_effort (字符串: minimal, low, medium, high)
	ThinkingBudget  int    `mapstructure:"thinking_budget"`
	ThinkingEffort  string `mapstructure:"thinking_effort"`
	ThinkingEnabled bool   `mapstructure:"thinking_enabled"`
}

// GetBaseURL 获取 Base URL（支持自动选择）
func (c *LLMConfig) GetBaseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return ProviderBaseURLs[string(c.Provider)]
}
