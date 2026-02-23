package tools

import (
	"context"
	"encoding/json"

	"charm.land/fantasy"
)

// AgentToolAdapter 将我们的 Tool 适配为 fantasy.AgentTool
type AgentToolAdapter struct {
	tool  Tool
	info  fantasy.ToolInfo
	opts  fantasy.ProviderOptions
}

// NewAgentToolAdapter 创建一个新的 fantasy.AgentTool 适配器
func NewAgentToolAdapter(tool Tool) fantasy.AgentTool {
	// 解析完整的 JSON Schema
	var fullSchema map[string]any
	if err := json.Unmarshal(tool.Parameters(), &fullSchema); err != nil {
		// 如果解析失败，使用空 map
		fullSchema = make(map[string]any)
	}

	// 提取 properties（ToolInfo.Parameters 应该只包含参数定义，不是完整 schema）
	properties := make(map[string]any)
	if props, ok := fullSchema["properties"].(map[string]any); ok {
		properties = props
	}

	// 提取必填字段（支持 []string 和 []any 两种类型）
	required := []string{}
	if req, ok := fullSchema["required"].([]string); ok {
		required = req
	} else if reqAny, ok := fullSchema["required"].([]any); ok {
		// JSON 解码可能将 []string 解析为 []any
		required = make([]string, 0, len(reqAny))
		for _, r := range reqAny {
			if s, ok := r.(string); ok {
				required = append(required, s)
			}
		}
	}

	return &AgentToolAdapter{
		tool: tool,
		info: fantasy.ToolInfo{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  properties,
			Required:    required,
			Parallel:    false,
		},
		opts: make(fantasy.ProviderOptions),
	}
}

// Info 返回工具信息
func (a *AgentToolAdapter) Info() fantasy.ToolInfo {
	return a.info
}

// Run 执行工具
func (a *AgentToolAdapter) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	// 解析输入参数 (从 JSON 字符串转换为 map[string]any)
	var input map[string]any
	if err := json.Unmarshal([]byte(call.Input), &input); err != nil {
		return fantasy.NewTextErrorResponse("failed to parse input: " + err.Error()), nil
	}

	// 验证输入
	if err := a.tool.Validate(input); err != nil {
		return fantasy.NewTextErrorResponse("validation failed: " + err.Error()), nil
	}

	// 执行工具
	result, err := a.tool.Execute(ctx, input)
	if err != nil {
		return fantasy.NewTextErrorResponse("execution failed: " + err.Error()), nil
	}

	return fantasy.NewTextResponse(result), nil
}

// ProviderOptions 返回 provider 选项
func (a *AgentToolAdapter) ProviderOptions() fantasy.ProviderOptions {
	return a.opts
}

// SetProviderOptions 设置 provider 选项
func (a *AgentToolAdapter) SetProviderOptions(opts fantasy.ProviderOptions) {
	a.opts = opts
}

// ToolsToAgentTools 将多个 Tool 转换为 fantasy.AgentTool 列表
func ToolsToAgentTools(tools []Tool) []fantasy.AgentTool {
	agentTools := make([]fantasy.AgentTool, len(tools))
	for i, tool := range tools {
		agentTools[i] = NewAgentToolAdapter(tool)
	}
	return agentTools
}
