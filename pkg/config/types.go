package config

// Config 应用配置
type Config struct {
	// LLM 配置
	LLM LLMConfig `mapstructure:",squash"`

	// 上下文管理配置
	Context ContextConfig `mapstructure:",squash"`

	// 工具配置
	Tool ToolConfig `mapstructure:",squash"`

	// ReAct 循环配置
	ReAct ReActConfig `mapstructure:",squash"`

	// 压缩配置
	Compressor      CompressorConfig      `mapstructure:",squash"`
	CompressWorker  CompressWorkerConfig  `mapstructure:",squash"`

	// 记忆服务配置
	Memory MemoryServiceConfig `mapstructure:",squash"`

	// UI 配置
	UI UIConfig `mapstructure:",squash"`

	// 路径配置
	WorkDir    string `mapstructure:"work_dir"`
	PromptDir  string `mapstructure:"prompt_dir"`
	ArchiveDir string `mapstructure:"archive_dir"`
}
