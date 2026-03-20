package context

import (
	"time"

	"github.com/yourname/oxencode/pkg/config"
)

// PageType 页面类型，表示上下文层级
type PageType string

const (
	PageTypeL0 PageType = "l0" // 全局高层次压缩
	PageTypeL1 PageType = "l1" // 轻度压缩的交互轮次
	PageTypeL2 PageType = "l2" // 原始 messages
)

// PageID 页面唯一标识
type PageID string

// SessionState Session状态
type SessionState int

const (
	StateNormal      SessionState = iota // 正常状态
	StateCompressing                     // L1正在压缩中
)

// Thresholds 阈值配置（计算后的绝对token值）
type Thresholds struct {
	HardMaxL0 int // L0 硬上限
	HardMaxL1 int // L1 硬上限
	SoftMaxL1 int // L1 软上限
	HardMaxL2 int // L2 硬上限
	SoftMaxL2 int // L2 软上限
}

// NewThresholds 从配置计算阈值（绝对token值）
func NewThresholds(cfg *config.Config) Thresholds {
	// 默认值
	maxContext := 200000
	hardMaxL0Percent := 0.1
	hardMaxL1Percent := 0.6
	softMaxL1Ratio := 0.6
	hardMaxL2Percent := 0.3
	softMaxL2Ratio := 0.7

	if cfg != nil {
		maxContext = cfg.Context.MaxContextTokens
		hardMaxL0Percent = cfg.Context.HardMaxL0Percent
		hardMaxL1Percent = cfg.Context.HardMaxL1Percent
		softMaxL1Ratio = cfg.Context.SoftMaxL1Ratio
		hardMaxL2Percent = cfg.Context.HardMaxL2Percent
		softMaxL2Ratio = cfg.Context.SoftMaxL2Ratio
	}

	hardMaxL0 := int(float64(maxContext) * hardMaxL0Percent)
	hardMaxL1 := int(float64(maxContext) * hardMaxL1Percent)
	hardMaxL2 := int(float64(maxContext) * hardMaxL2Percent)

	return Thresholds{
		HardMaxL0: hardMaxL0,
		HardMaxL1: hardMaxL1,
		SoftMaxL1: int(float64(hardMaxL1) * softMaxL1Ratio),
		HardMaxL2: hardMaxL2,
		SoftMaxL2: int(float64(hardMaxL2) * softMaxL2Ratio),
	}
}

// CompressionStrategy 压缩策略配置
type CompressionStrategy struct {
	// Skill 配置
	Skill string `json:"skill"` // 用于特定压缩任务的 skill

	// 压缩模型标识
	CompressionModel string `json:"compression_model"` // 用于压缩的模型标识

	// 超时配置
	Timeout time.Duration `json:"timeout"` // 压缩超时时间

	// 截断配置（从Config读取）
	MaxToolOutputLength int `json:"max_tool_output_length"` // 工具输出最大长度，0表示不截断
	MaxAssistantLength  int `json:"max_assistant_length"`   // Assistant消息最大长度，0表示不截断
}

// NewCompressionStrategy 从Config创建指定PageType的压缩策略
// 注意：Skill 需要在调用方设置，因为需要从 prompt 包加载
func NewCompressionStrategy(pageType PageType, cfg *config.Config) *CompressionStrategy {
	switch pageType {
	case PageTypeL1:
		return &CompressionStrategy{
			Timeout:             30 * time.Second,
			MaxToolOutputLength: cfg.Context.L1MaxToolOutput,
			MaxAssistantLength:  cfg.Context.L1MaxAssistant,
		}
	case PageTypeL0:
		return &CompressionStrategy{
			Timeout:             60 * time.Second,
			MaxToolOutputLength: 0, // 不截断，LLM处理
			MaxAssistantLength:  0,
		}
	case PageTypeL2:
		return &CompressionStrategy{
			Timeout:             5 * time.Second,
			MaxToolOutputLength: 0, // 不截断
			MaxAssistantLength:  0,
		}
	}
	return nil
}

// DefaultCompressionStrategies 返回默认的压缩策略配置（使用默认Config）
// 注意：Skill 需要在调用方设置
func DefaultCompressionStrategies() (L0, L1, L2 *CompressionStrategy) {
	cfg := config.Get()
	if cfg == nil {
		// 如果没有配置，使用硬编码默认值
		L1 = &CompressionStrategy{
			Timeout:             30 * time.Second,
			MaxToolOutputLength: 1000,
			MaxAssistantLength:  2000,
		}

		L0 = &CompressionStrategy{
			Timeout:             60 * time.Second,
			MaxToolOutputLength: 0,
			MaxAssistantLength:  0,
		}

		L2 = &CompressionStrategy{
			Timeout:             5 * time.Second,
			MaxToolOutputLength: 0,
			MaxAssistantLength:  0,
		}
		return L0, L1, L2
	}

	return NewCompressionStrategy(PageTypeL0, cfg),
		NewCompressionStrategy(PageTypeL1, cfg),
		NewCompressionStrategy(PageTypeL2, cfg)
}