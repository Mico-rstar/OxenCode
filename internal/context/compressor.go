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
	agent  fantasy.Agent
	logger logger.Logger
	cfg    *config.Config
}

// NewLLMCompressor 创建 LLM 压缩器
// 系统提示词在此处加载完成（包含 skill 注入）
func NewLLMCompressor(ctx context.Context, provider fantasy.Provider, strategy *CompressionStrategy, cfg *config.Config, lg logger.Logger, prt *prompt.Prompt) (*LLMCompressor, error) {

	// 从全局配置读取模型配置
	model := cfg.Compressor.Model
	if model == "" {
		panic("CompressorModel is needed in config")
	}

	llm, err := provider.LanguageModel(ctx, model)
	if err != nil {
		return nil, fmt.Errorf("failed to create language model: %w", err)
	}

	maxTokens := cfg.Compressor.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	temperature := cfg.Compressor.Temperature
	if temperature == 0 {
		temperature = 0.3
	}

	// 构造系统提示词，注入 skill
	crpPrt := prt.CompressorSystemPrompt
	crpPrt = prompt.InjectVariables(crpPrt, map[string]string{
		"lx_skill": strategy.Skill,
	})

	agent := fantasy.NewAgent(llm,
		fantasy.WithSystemPrompt(crpPrt),
		fantasy.WithMaxOutputTokens(maxTokens),
		fantasy.WithTemperature(temperature),
	)

	return &LLMCompressor{
		agent:  agent,
		logger: lg,
		cfg:    cfg,
	}, nil
}

// getLogger 获取 logger，如果为 nil 则返回 nop logger
func (c *LLMCompressor) getLogger() logger.Logger {
	if c.logger != nil {
		return c.logger
	}
	return logger.NewNop()
}

// Compress 实现 Compressor 接口，单轮压缩
// 系统提示词已在 NewLLMCompressor 中配置完成，只需传入原始内容
func (c *LLMCompressor) Compress(ctx context.Context, raw string, strategy *CompressionStrategy) (string, error) {
	// 超时控制
	if strategy.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, strategy.Timeout)
		defer cancel()
	}

	c.getLogger().Debug("Starting compression", "input_length", len(raw))

	// 直接调用 LLM，系统提示词已在 agent 创建时配置
	result, err := c.agent.Generate(ctx, fantasy.AgentCall{
		Prompt: raw, // 只传入原始内容
	})
	if err != nil {
		return "", fmt.Errorf("compression failed: %w", err)
	}

	// 提取响应内容
	compressed := ""
	for _, content := range result.Response.Content {
		if text, ok := content.(fantasy.TextContent); ok {
			compressed += text.Text
		}
	}

	// 使用 prompt.ExtractVariables 从 <output>...</output> 标签中提取内容
	vars := prompt.ExtractVariables(compressed)
	if output, ok := vars["output"]; ok {
		c.getLogger().Debug("Compression completed",
			"input_length", len(raw),
			"output_length", len(output))
		return output, nil
	}

	// 没有找到 output 标签，返回原始内容
	c.getLogger().Warn("No <output> tag found in compression result, returning raw response")
	return compressed, nil
}
