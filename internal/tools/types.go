package tools

// Tool 工具接口
type Tool interface {
	// Name 返回工具名称
	Name() string

	// Description 返回工具描述
	Description() string

	// Execute 执行工具
	Execute(params map[string]interface{}) (string, error)
}

// ToolResult 工具执行结果
type ToolResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}
