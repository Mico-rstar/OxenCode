package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"charm.land/fantasy"
	"github.com/yourname/oxencode/internal/tools"
	"github.com/yourname/oxencode/pkg/config"
	"github.com/yourname/oxencode/pkg/logger"
)

// ToolExecutor 工具执行器
// 负责执行工具调用并对输出进行截断
type ToolExecutor struct {
	registry *tools.Registry
	env      tools.Environment
	config   *config.Config
	logger   logger.Logger
}

// NewToolExecutor 创建工具执行器
func NewToolExecutor(registry *tools.Registry, env tools.Environment, config *config.Config, logger logger.Logger) *ToolExecutor {
	return &ToolExecutor{
		registry: registry,
		env:      env,
		config:   config,
		logger:   logger,
	}
}

// ToolResult 工具执行结果
type ToolResult struct {
	Output  string
	Error   string
	IsError bool
}

// Execute 执行工具调用
func (e *ToolExecutor) Execute(ctx context.Context, tc fantasy.ToolCallContent) (*ToolResult, error) {
	// 1. 获取工具
	tool := e.registry.Get(tc.ToolName)
	if tool == nil {
		return nil, fmt.Errorf("tool not found: %s", tc.ToolName)
	}

	// 2. 解析输入
	var input map[string]any
	if tc.Input != "" {
		if err := json.Unmarshal([]byte(tc.Input), &input); err != nil {
			return nil, fmt.Errorf("invalid tool input: %w", err)
		}
	} else {
		input = make(map[string]any)
	}

	// 3. 验证参数
	if err := tool.Validate(input); err != nil {
		return &ToolResult{
			Output:  "",
			Error:   fmt.Sprintf("validation failed: %v", err),
			IsError: true,
		}, nil
	}

	// 4. 执行工具
	e.logger.Info("Executing tool", "tool", tc.ToolName, "callID", tc.ToolCallID)
	output, err := tool.Execute(ctx, input)
	if err != nil {
		e.logger.Error("Tool execution failed", "tool", tc.ToolName, "error", err)
		return &ToolResult{
			Output:  "",
			Error:   err.Error(),
			IsError: true,
		}, nil
	}

	// 5. 关键：截断输出
	output = e.truncateOutput(output, tc.ToolName)


	return &ToolResult{
		Output:  output,
		IsError: false,
	}, nil
}

// truncateOutput 截断工具输出
// 这是防止上下文爆炸的关键措施
func (e *ToolExecutor) truncateOutput(output, toolName string) string {
	maxLen := e.getToolOutputMaxLength()

	if len(output) <= maxLen {
		return output
	}

	e.logger.Warn("Tool output truncated",
		"tool", toolName,
		"original", len(output),
		"truncated", maxLen)

	// 截断并添加提示
	return output[:maxLen] + fmt.Sprintf("\n\n[...truncated, original length: %d bytes]\n[Use Grep to search for specific patterns in the output]", len(output))
}

// getToolOutputMaxLength 获取工具输出最大长度
func (e *ToolExecutor) getToolOutputMaxLength() int {
	// 从配置读取，默认 10KB
	if e.config != nil && e.config.ToolOutputMaxLength > 0 {
		return e.config.ToolOutputMaxLength
	}
	return 10000
}

// ExecuteWithRawOutput 执行工具但不截断（用于特殊场景）
func (e *ToolExecutor) ExecuteWithRawOutput(ctx context.Context, tc fantasy.ToolCallContent) (*ToolResult, error) {
	// 获取工具
	tool := e.registry.Get(tc.ToolName)
	if tool == nil {
		return nil, fmt.Errorf("tool not found: %s", tc.ToolName)
	}

	// 解析输入
	var input map[string]any
	if tc.Input != "" {
		if err := json.Unmarshal([]byte(tc.Input), &input); err != nil {
			return nil, fmt.Errorf("invalid tool input: %w", err)
		}
	} else {
		input = make(map[string]any)
	}

	// 验证参数
	if err := tool.Validate(input); err != nil {
		return &ToolResult{
			Output:  "",
			Error:   fmt.Sprintf("validation failed: %v", err),
			IsError: true,
		}, nil
	}

	// 执行工具（不截断）
	output, err := tool.Execute(ctx, input)
	if err != nil {
		return &ToolResult{
			Output:  "",
			Error:   err.Error(),
			IsError: true,
		}, nil
	}

	return &ToolResult{
		Output:  output,
		IsError: false,
	}, nil
}