package context

import (
	"context"
	"fmt"

	"charm.land/fantasy"
	"github.com/yourname/oxencode/internal/prompt"
	"github.com/yourname/oxencode/pkg/logger"
)

// Compressor 压缩器接口
type Compressor interface {
	// Compress 压缩原始内容
	// ctx: 上下文，用于控制超时
	// raw: 原始内容（通常是序列化的 messages）
	// strategy: 压缩策略配置
	// 返回：压缩后的内容
	Compress(ctx context.Context, raw string, strategy *CompressionStrategy) (string, error)
}

// LLMCompressor 使用 LLM 进行文本压缩的实现
type LLMCompressor struct {
	agent  fantasy.Agent
	logger logger.Logger
}

// LLMCompressorConfig LLMCompressor 配置
type LLMCompressorConfig struct {
	Model string
}

// NewLLMCompressor 创建 LLM 压缩器
func NewLLMCompressor(ctx context.Context, provider fantasy.Provider, config *LLMCompressorConfig) (*LLMCompressor, error) {
	log := logger.New("context/compressor")

	model := config.Model
	if model == "" {
		model = "claude-sonnet-4-5-20250514" // 默认使用较快的模型
	}

	llm, err := provider.LanguageModel(ctx, model)
	if err != nil {
		return nil, fmt.Errorf("failed to create language model: %w", err)
	}

	// 创建压缩专用的 agent
	// 使用简化的系统提示词，专注于压缩任务
	systemPrompt := `You are a text compression assistant. Your task is to compress input text according to the given schema while preserving key information.

Guidelines:
- Extract and preserve key information (code snippets, configurations, important findings)
- Remove redundant details, verbose logs, and low-density information
- Follow the schema structure strictly
- Keep the compressed content concise but complete
- Maintain readability and coherence`

	agent := fantasy.NewAgent(llm,
		fantasy.WithSystemPrompt(systemPrompt),
		fantasy.WithMaxOutputTokens(4096),
		fantasy.WithTemperature(0.3), // 较低的 temperature 确保更确定的输出
	)

	return &LLMCompressor{
		agent:  agent,
		logger: log,
	}, nil
}

// Compress 实现 Compressor 接口
func (c *LLMCompressor) Compress(ctx context.Context, raw string, strategy *CompressionStrategy) (string, error) {
	c.logger.Debug("Compressing content", "raw_length", len(raw), "schema_length", len(strategy.Schema))

	// 构建压缩提示词
	promptText := fmt.Sprintf(`Please compress the following content according to this schema:

## Schema
%s

## Input Content
%s

## Compressed Output
`, strategy.Schema, raw)

	// 调用 LLM 进行压缩
	result, err := c.agent.Generate(ctx, fantasy.AgentCall{
		Prompt: promptText,
	})

	if err != nil {
		c.logger.Error("LLM compression failed", "error", err)
		return "", fmt.Errorf("LLM compression failed: %w", err)
	}

	// 提取响应内容
	if result == nil || len(result.Response.Content) == 0 {
		return "", fmt.Errorf("empty compression result")
	}

	var compressed string
	for _, c := range result.Response.Content {
		if text, ok := c.(fantasy.TextContent); ok {
			compressed += text.Text
		}
	}

	// 验证压缩率
	compressionRate := float64(len(compressed)) / float64(len(raw))
	c.logger.Debug("Compression complete",
		"raw_length", len(raw),
		"compressed_length", len(compressed),
		"compression_rate", compressionRate,
		"min_rate", strategy.MinCompressionRate,
		"max_rate", strategy.MaxCompressionRate,
	)

	if compressionRate > strategy.MaxCompressionRate {
		c.logger.Warn("Compression rate exceeded maximum",
			"rate", compressionRate,
			"max", strategy.MaxCompressionRate)
		// 压缩率过高，尝试进一步压缩或警告
	}

	if compressionRate < strategy.MinCompressionRate {
		c.logger.Warn("Compression rate below minimum",
			"rate", compressionRate,
			"min", strategy.MinCompressionRate)
		// 压缩率过低，可能丢失信息
	}

	return compressed, nil
}

// MockCompressor 用于测试的模拟压缩器
type MockCompressor struct {
	MaxOutputLength int
}

// NewMockCompressor 创建模拟压缩器
func NewMockCompressor(maxOutputLength int) *MockCompressor {
	return &MockCompressor{
		MaxOutputLength: maxOutputLength,
	}
}

// Compress 实现 Compressor 接口（模拟版本）
func (c *MockCompressor) Compress(ctx context.Context, raw string, strategy *CompressionStrategy) (string, error) {
	// 简单截断作为模拟压缩
	if len(raw) <= c.MaxOutputLength {
		return raw, nil
	}

	// 使用 prompt loader 提取 schema 的关键信息作为前缀
	schemaPreview := ""
	if strategy.Schema != "" {
		lines := prompt.SplitLines(strategy.Schema)
		if len(lines) > 5 {
			lines = lines[:5]
		}
		for _, line := range lines {
			schemaPreview += line + "\n"
		}
	}

	return fmt.Sprintf("[Compressed]\nSchema:\n%s\n\n... (truncated from %d to %d chars)",
		schemaPreview, len(raw), c.MaxOutputLength), nil
}
