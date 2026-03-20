package config

import (
	"os"

	"github.com/yourname/oxencode/pkg/logger"
)

// GetAPIKeyFromEnv 从环境变量获取 API Key
func (c *Config) GetAPIKeyFromEnv() string {
	log := logger.New("config")

	if c.LLM.APIKey != "" {
		log.Debug("Using API key from config")
		return c.LLM.APIKey
	}

	// 根据 provider 尝试不同的环境变量
	log.Debug("Trying to get API key from environment", "provider", c.LLM.Provider)

	switch c.LLM.Provider {
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
