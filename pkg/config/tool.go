package config

// ToolConfig 工具执行配置
type ToolConfig struct {
	Timeout    int `mapstructure:"tool_timeout"`
	MaxRetries int `mapstructure:"tool_max_retries"`
}

// ReActConfig ReAct 循环配置
type ReActConfig struct {
	MaxIterations     int `mapstructure:"max_react_iterations"`
	ToolOutputMaxLength int `mapstructure:"tool_output_max_length"`
}
