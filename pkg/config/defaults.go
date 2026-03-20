package config

// === LLM Provider 默认值 ===
const (
	DefaultProvider          = "anthropic"
	DefaultModel             = "claude-sonnet-4-5-20250514"
	DefaultMaxTokens         = 8192
	DefaultTemperature       = 0.7
	DefaultThinkingBudget    = 0
	DefaultThinkingEnabled   = false
)

// ProviderBaseURLs 各 Provider 的默认 Base URL
var ProviderBaseURLs = map[string]string{
	"qwen":     "https://dashscope.aliyuncs.com/compatible-mode/v1",
	"deepseek": "https://api.deepseek.com",
	"glm":      "https://open.bigmodel.cn/api/paas/v4",
}

// === 工具执行默认值 ===
const (
	DefaultToolTimeout    = 120 // 秒
	DefaultToolMaxRetries = 3
)

// === 上下文管理默认值 ===
const (
	DefaultMaxContextTokens  = 75000
	DefaultMaxPageTokens     = 10000
	DefaultHardMaxL0Percent  = 0.1
	DefaultHardMaxL1Percent  = 0.6
	DefaultSoftMaxL1Ratio    = 0.6
	DefaultHardMaxL2Percent  = 0.3
	DefaultSoftMaxL2Ratio    = 0.7
	DefaultL1MaxToolLength   = 1000
	DefaultL1MaxAssistantLen = 2000
)

// === 压缩策略默认值 ===
const (
	DefaultCompressorModel       = "claude-sonnet-4-5-20250514"
	DefaultCompressorMaxTokens   = 4096
	DefaultCompressorTemperature = 0.3
	DefaultCompressorMaxRetries  = 3

	// 各层级压缩超时（秒）
	DefaultL0CompressTimeout = 60
	DefaultL1CompressTimeout = 30
	DefaultL2CompressTimeout = 5
)

// === 压缩工作器默认值 ===
const (
	DefaultCompressWorkerCount = 2
	DefaultCompressQueueSize   = 50
)

// === 记忆服务默认值 ===
const (
	DefaultMemoryServiceURL       = "http://127.0.0.1:8765"
	DefaultMemoryServiceTimeout   = 30  // 秒
	DefaultMemoryMaxRetries       = 3
	DefaultMemoryRetryInterval    = 1   // 秒
	DefaultMemoryMonitorPoll      = 10  // 秒
	DefaultMemoryMonitorRetries   = 3
	DefaultMemoryMonitorTimeout   = 600 // 秒
	DefaultMemoryTriggerThreshold = 0.5
)

// === ReAct 循环默认值 ===
const (
	DefaultMaxReActIterations  = 50
	DefaultToolOutputMaxLength = 10000
)

// === UI 样式默认值 ===
var UIColors = struct {
	Primary   string
	Secondary string
	Muted     string
	Error     string
	Warning   string
	Success   string
}{
	Primary:   "86",  // 青色
	Secondary: "98",  // 紫色
	Muted:     "245", // 灰色
	Error:     "196", // 红色
	Warning:   "226", // 黄色
	Success:   "142", // 绿色
}

// === 路径默认值 ===
const (
	DefaultWorkDir   = "."
	DefaultPromptDir = "pkg/prompt/prompts"
	DefaultArchiveDir = "archives"
)
