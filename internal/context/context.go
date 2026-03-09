package context

import (
	"context"
	"fmt"

	"charm.land/fantasy"
	"github.com/yourname/oxencode/pkg/config"
	"github.com/yourname/oxencode/pkg/logger"
	"github.com/yourname/oxencode/pkg/prompt"
)

// NewLLMCompressorWithProvider 创建 LLM 压缩器（需要传入 fantasy provider）
// 注意：仅用于L0压缩，L1使用Preprocess而非LLM压缩
func NewLLMCompressorWithProvider(ctx context.Context, provider fantasy.Provider, cfg *config.Config, log logger.Logger) (Compressor, error) {
	// 加载提示词（使用配置中的 promptDir）
	promptDir := cfg.PromptDir
	if promptDir == "" {
		promptDir = "pkg/prompt/prompts" // 默认值
	}
	prt := prompt.New(promptDir)
	if err := prt.Load(); err != nil {
		return nil, fmt.Errorf("failed to load prompt: %w", err)
	}

	// 创建L0策略，使用 prompt 包的 skill
	l0Strategy := NewCompressionStrategy(PageTypeL0, cfg)
	l0Strategy.Skill = prt.L0Skill

	return NewLLMCompressor(ctx, provider, l0Strategy, cfg, log, prt)
}