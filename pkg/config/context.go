package config

// ContextConfig 上下文管理配置
type ContextConfig struct {
	// 全局上限
	MaxContextTokens int `mapstructure:"max_context_tokens"`
	MaxPageTokens    int `mapstructure:"max_page_tokens"`

	// L0 配置（全局高层次压缩）
	HardMaxL0Percent float64 `mapstructure:"hard_max_l0_percent"`

	// L1 配置（轻度压缩）
	HardMaxL1Percent float64 `mapstructure:"hard_max_l1_percent"`
	SoftMaxL1Ratio   float64 `mapstructure:"soft_max_l1_ratio"`
	L1MaxToolOutput  int     `mapstructure:"l1_max_tool_output_length"`
	L1MaxAssistant   int     `mapstructure:"l1_max_assistant_length"`

	// L2 配置（原始 messages）
	HardMaxL2Percent float64 `mapstructure:"hard_max_l2_percent"`
	SoftMaxL2Ratio   float64 `mapstructure:"soft_max_l2_ratio"`

	// 压缩超时
	CompressTimeout int `mapstructure:"compress_timeout"`
}

// Thresholds 阈值（计算后的绝对值）
type Thresholds struct {
	HardMaxL0 int
	HardMaxL1 int
	SoftMaxL1 int
	HardMaxL2 int
	SoftMaxL2 int
}

// ComputeThresholds 计算绝对阈值
func (c *ContextConfig) ComputeThresholds() Thresholds {
	hardMaxL0 := int(float64(c.MaxContextTokens) * c.HardMaxL0Percent)
	hardMaxL1 := int(float64(c.MaxContextTokens) * c.HardMaxL1Percent)
	hardMaxL2 := int(float64(c.MaxContextTokens) * c.HardMaxL2Percent)

	return Thresholds{
		HardMaxL0: hardMaxL0,
		HardMaxL1: hardMaxL1,
		SoftMaxL1: int(float64(hardMaxL1) * c.SoftMaxL1Ratio),
		HardMaxL2: hardMaxL2,
		SoftMaxL2: int(float64(hardMaxL2) * c.SoftMaxL2Ratio),
	}
}
