package tools

import (
	"context"
	"encoding/json"

	"github.com/yourname/oxencode/pkg/logger"
)

// Tool 工具接口
// 每个工具需要实现此接口以便被 ToolExecutor 调用
type Tool interface {
	// Name 返回工具名称（唯一标识）
	// 名称必须是唯一的，用于在工具注册表中查找工具
	Name() string

	// Description 返回工具描述（用于生成 schema）
	// 描述会被传递给 LLM，帮助 LLM 理解工具的用途
	Description() string

	// Parameters 返回参数 schema (JSON Schema 格式)
	// 定义工具接受的输入参数结构
	Parameters() json.RawMessage

	// Execute 执行工具
	// ctx: 上下文，用于取消和超时控制
	// input: 工具输入参数，map[string]any 类型
	// 返回: 工具执行结果（字符串形式）和错误信息
	Execute(ctx context.Context, input map[string]any) (string, error)

	// Validate 验证输入参数（可选，默认使用 schema 验证）
	// 如果工具实现了自定义验证逻辑，可以重写此方法
	Validate(input map[string]any) error
}

// BaseTool 基础工具实现
// 提供了默认的 Validate 实现，使用 JSON Schema 进行参数验证
type BaseTool struct {
	name        string
	description string
	parameters  json.RawMessage
	env         Environment
	logger      logger.Logger
}

// NewBaseTool 创建基础工具
// logger 是可选的，如果传入 nil 则创建基于全局 logger 的实例
func NewBaseTool(name, description string, parameters json.RawMessage, env Environment, log logger.Logger) *BaseTool {
	if log == nil {
		log = logger.New("tool." + name)
	} else {
		log = log.Named("tool." + name)
	}
	return &BaseTool{
		name:        name,
		description: description,
		parameters:  parameters,
		env:         env,
		logger:      log,
	}
}

// Name 返回工具名称
func (t *BaseTool) Name() string {
	return t.name
}

// Description 返回工具描述
func (t *BaseTool) Description() string {
	return t.description
}

// Parameters 返回参数 schema
func (t *BaseTool) Parameters() json.RawMessage {
	return t.parameters
}

// Validate 默认的参数验证实现
// 基础实现只检查必填字段是否存在，具体工具可以重写此方法
func (t *BaseTool) Validate(input map[string]any) error {
	t.logger.Debug("Validating input", "input", input)

	// 解析 schema 提取必填字段
	var schema struct {
		Required []string `json:"required"`
	}

	if err := json.Unmarshal(t.parameters, &schema); err != nil {
		t.logger.Warn("Failed to parse schema for validation", "error", err)
		// 如果无法解析 schema，跳过验证
		return nil
	}

	// 检查必填字段
	for _, field := range schema.Required {
		if _, exists := input[field]; !exists {
			t.logger.Warn("Missing required field", "field", field)
			return &ValidationError{
				Field:   field,
				Message: "required field missing",
			}
		}
	}

	t.logger.Debug("Validation passed")
	return nil
}

// ValidationError 参数验证错误
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// ToolExecuteError 工具执行错误
type ToolExecuteError struct {
	ToolName string
	Err      error
}

func (e *ToolExecuteError) Error() string {
	return e.Err.Error()
}

func (e *ToolExecuteError) Unwrap() error {
	return e.Err
}
