package agent

import (
	"context"
	"testing"

	"github.com/yourname/oxencode/internal/config"
)

// TestQwenProvider 测试 Qwen provider 创建
func TestQwenProvider(t *testing.T) {
	apiKey := "sk-test-key"
	cfg := &config.Config{
		Provider:    config.ProviderQwen,
		Model:       "qwen-max",
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	provider, err := createProvider(cfg, apiKey)
	if err != nil {
		t.Fatalf("createProvider() error = %v", err)
	}

	if provider == nil {
		t.Fatal("createProvider() returned nil provider")
	}

	// 测试获取 language model
	model, err := provider.LanguageModel(context.Background(), cfg.Model)
	if err != nil {
		t.Fatalf("provider.LanguageModel() error = %v", err)
	}

	if model == nil {
		t.Fatal("provider.LanguageModel() returned nil model")
	}

	t.Logf("Successfully created Qwen provider and model: %v", model)
}

// TestOpenAIProvider 测试 OpenAI provider 创建
func TestOpenAIProvider(t *testing.T) {
	apiKey := "sk-test-key"
	cfg := &config.Config{
		Provider:    config.ProviderOpenAI,
		Model:       "gpt-4",
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	provider, err := createProvider(cfg, apiKey)
	if err != nil {
		t.Fatalf("createProvider() error = %v", err)
	}

	if provider == nil {
		t.Fatal("createProvider() returned nil provider")
	}

	// 测试获取 language model
	model, err := provider.LanguageModel(context.Background(), cfg.Model)
	if err != nil {
		t.Fatalf("provider.LanguageModel() error = %v", err)
	}

	if model == nil {
		t.Fatal("provider.LanguageModel() returned nil model")
	}

	t.Logf("Successfully created OpenAI provider and model: %v", model)
}

// TestDeepSeekProvider 测试 DeepSeek provider 创建
func TestDeepSeekProvider(t *testing.T) {
	apiKey := "sk-test-key"
	cfg := &config.Config{
		Provider:    config.ProviderDeepSeek,
		Model:       "deepseek-chat",
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	provider, err := createProvider(cfg, apiKey)
	if err != nil {
		t.Fatalf("createProvider() error = %v", err)
	}

	if provider == nil {
		t.Fatal("createProvider() returned nil provider")
	}

	// 测试获取 language model
	model, err := provider.LanguageModel(context.Background(), cfg.Model)
	if err != nil {
		t.Fatalf("provider.LanguageModel() error = %v", err)
	}

	if model == nil {
		t.Fatal("provider.LanguageModel() returned nil model")
	}

	t.Logf("Successfully created DeepSeek provider and model: %v", model)
}
