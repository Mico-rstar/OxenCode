package context

import (
	"context"
	"fmt"

	"charm.land/fantasy"
	"github.com/yourname/oxencode/pkg/config"
	"github.com/yourname/oxencode/pkg/logger"
	"github.com/yourname/oxencode/pkg/prompt"
)

// Compressor 压缩器接口（仅用于L0压缩）
type Compressor interface {
	// Compress 压缩原始内容
	// ctx: 上下文，用于控制超时
	// raw: 原始内容（通常是序列化的 messages）
	// strategy: 压缩策略配置
	// 返回：压缩后的内容
	Compress(ctx context.Context, raw string, strategy *CompressionStrategy) (string, error)
}

// LLMCompressor 使用 LLM 进行文本压缩的实现（仅用于L0压缩）
type LLMCompressor struct {
	agent    fantasy.Agent
	logger   logger.Logger
	cfg      *config.Config
	strategy *CompressionStrategy
}

// NewLLMCompressor 创建 LLM 压缩器
func NewLLMCompressor(ctx context.Context, provider fantasy.Provider, strategy *CompressionStrategy, cfg *config.Config, lg logger.Logger, prt *prompt.Prompt) (*LLMCompressor, error) {

	// 从全局配置读取模型配置
	model := cfg.CompressorModel
	if model == "" {
		panic("Compressed model is needed")
	}

	llm, err := provider.LanguageModel(ctx, model)
	if err != nil {
		return nil, fmt.Errorf("failed to create language model: %w", err)
	}

	maxTokens := cfg.CompressorMaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	temperature := cfg.CompressorTemperature
	if temperature == 0 {
		temperature = 0.3
	}

	// 构造系统提示词
	crpPrt := prt.CompressorSystemPrompt
	crpPrt = prompt.InjectVariables(crpPrt, map[string]string{
		"skill": strategy.Skill,
	})

	agent := fantasy.NewAgent(llm,
		fantasy.WithSystemPrompt(crpPrt),
		fantasy.WithMaxOutputTokens(maxTokens),
		fantasy.WithTemperature(temperature),
	)

	return &LLMCompressor{
		agent:    agent,
		logger:   lg,
		cfg:      cfg,
		strategy: strategy,
	}, nil
}

// Compress 实现 Compressor 接口，带有 ReAct 循环和超时控制
func (c *LLMCompressor) Compress(ctx context.Context, raw string, strategy *CompressionStrategy) (string, error) {
	// 超时控制
	if strategy.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, strategy.Timeout)
		defer cancel()
	}

	// ReAct 循环：不断压缩直到压缩率满足要求
	var compressed string
	var lastError error

	for iteration := 0; iteration < c.cfg.CompressorMaxRetries; iteration++ {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("compression timeout: %w", ctx.Err())
		default:
		}

		// 构建压缩提示词
		promptText := fmt.Sprintf(`Please compress the following content according to this schema:

## Schema
%s

## Input Content
%s

## Compressed Output
`, strategy.Schema, raw)

		// 如果是重试，添加压缩率调整提示
		if iteration > 0 {
			promptText = fmt.Sprintf(`Previous compression attempt did not meet compression rate requirements. Please adjust accordingly.

## Schema
%s

## Input Content
%s

## Previous Compression Rate
%0.2f

## Rate Constraints
- Maximum allowed rate: %0.2f (compressed/original)
- Minimum required rate: %0.2f

%s

## Compressed Output
`,
				strategy.Schema,
				raw,
				float64(len(compressed))/float64(len(raw)),
				strategy.MaxCompressionRate,
				strategy.MinCompressionRate,
				buildRateAdjustmentPrompt(compressed, raw, strategy),
			)
		}

		// 调用 LLM 进行压缩
		result, err := c.agent.Generate(ctx, fantasy.AgentCall{
			Prompt: promptText,
		})

		if err != nil {
			c.logger.Error("LLM compression failed", "error", err, "iteration", iteration)
			lastError = err
			continue
		}

		// 提取响应内容
		if result == nil || len(result.Response.Content) == 0 {
			c.logger.Error("Empty compression result", "iteration", iteration)
			lastError = fmt.Errorf("empty compression result")
			continue
		}

		compressed = ""
		for _, content := range result.Response.Content {
			if text, ok := content.(fantasy.TextContent); ok {
				compressed += text.Text
			}
		}

		// 验证压缩率
		compressionRate := float64(len(compressed)) / float64(len(raw))
		c.logger.Debug("Compression attempt complete",
			"iteration", iteration,
			"raw_length", len(raw),
			"compressed_length", len(compressed),
			"compression_rate", compressionRate,
			"min_rate", strategy.MinCompressionRate,
			"max_rate", strategy.MaxCompressionRate,
		)

		// 检查压缩率是否在允许范围内
		if compressionRate <= strategy.MaxCompressionRate && compressionRate >= strategy.MinCompressionRate {
			c.logger.Info("Compression rate within acceptable range", "rate", compressionRate)
			return compressed, nil
		}

		// 压缩率不符合要求，继续重试
		if compressionRate > strategy.MaxCompressionRate {
			c.logger.Warn("Compression rate exceeded maximum, will retry",
				"rate", compressionRate, "max", strategy.MaxCompressionRate)
		} else {
			c.logger.Warn("Compression rate below minimum, will retry",
				"rate", compressionRate, "min", strategy.MinCompressionRate)
		}
	}

	// 达到最大重试次数，返回最后一次结果
	if lastError != nil {
		return "", fmt.Errorf("compression failed after %d attempts: %w", c.cfg.CompressorMaxRetries, lastError)
	}

	c.logger.Warn("Compression completed but rate constraints not met, returning best effort",
		"final_rate", float64(len(compressed))/float64(len(raw)))
	return compressed, nil
}

// buildRateAdjustmentPrompt 根据压缩率情况构建调整提示
func buildRateAdjustmentPrompt(compressed string, raw string, strategy *CompressionStrategy) string {
	compressionRate := float64(len(compressed)) / float64(len(raw))

	if compressionRate > strategy.MaxCompressionRate {
		return "Please compress FURTHER. Remove more redundant details, verbose logs, and low-density information. Be more aggressive in summarization while preserving key information."
	} else if compressionRate < strategy.MinCompressionRate {
		return "Please preserve MORE information. The compression is too aggressive. Include more details about agent actions, key findings, code snippets, and important context while still following the schema structure."
	}
	return ""
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

	// 使用 SplitLines 提取 schema 的关键信息作为前缀
	schemaPreview := ""
	if strategy.Schema != "" {
		lines := splitLines(strategy.Schema)
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

// splitLines 将文本分割为行（避免导入 prompt 包造成循环依赖）
func splitLines(text string) []string {
	result := []string{}
	current := ""
	for _, ch := range text {
		if ch == '\n' {
			result = append(result, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	result = append(result, current)
	return result
}