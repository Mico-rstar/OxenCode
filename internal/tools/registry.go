package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/yourname/oxencode/pkg/logger"
)

// Registry 工具注册表
// 管理所有可用的工具，提供注册、查找和列表功能
type Registry struct {
	mu     sync.RWMutex
	tools  map[string]Tool
	logger logger.Logger
}

// NewRegistry 创建新的工具注册表
// logger 是可选的，如果传入 nil 则创建基于全局 logger 的实例
func NewRegistry(log logger.Logger) *Registry {
	if log == nil {
		log = logger.New("registry")
	}
	return &Registry{
		tools:  make(map[string]Tool),
		logger: log,
	}
}

// Register 注册工具
// 如果同名工具已存在，会返回错误
func (r *Registry) Register(tool Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := tool.Name()

	if _, exists := r.tools[name]; exists {
		r.logger.Warn("Tool already registered", "name", name)
		return fmt.Errorf("tool already registered: %s", name)
	}

	r.tools[name] = tool
	r.logger.Info("Tool registered",
		"name", name,
		"description", tool.Description())

	return nil
}

// Unregister 注销工具
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; !exists {
		r.logger.Warn("Tool not found", "name", name)
		return fmt.Errorf("tool not found: %s", name)
	}

	delete(r.tools, name)
	r.logger.Info("Tool unregistered", "name", name)

	return nil
}

// Get 获取工具
func (r *Registry) Get(name string) Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.tools[name]
}

// Has 检查工具是否存在
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.tools[name]
	return exists
}

// List 返回所有工具
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}

	return result
}

// Names 返回所有工具名称
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}

	return names
}

// Count 返回工具数量
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.tools)
}

// GetToolSchemas 获取所有工具的 schema（用于传递给 LLM）
// 返回格式符合 Claude/Anthropic 的 function calling 格式
func (r *Registry) GetToolSchemas() []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	schemas := make([]map[string]any, 0, len(r.tools))

	for _, tool := range r.tools {
		// 解析参数 schema
		var params map[string]any
		if err := json.Unmarshal(tool.Parameters(), &params); err != nil {
			r.logger.Warn("Failed to parse tool parameters",
				"tool", tool.Name(),
				"error", err)
			continue
		}

		schema := map[string]any{
			"name":        tool.Name(),
			"description": tool.Description(),
			"input_schema": params,
		}
		schemas = append(schemas, schema)
	}

	return schemas
}

// Execute 执行工具
// name: 工具名称
// ctx: 上下文
// input: 输入参数
func (r *Registry) Execute(ctx context.Context, name string, input map[string]any) (string, error) {
	tool := r.Get(name)
	if tool == nil {
		return "", fmt.Errorf("tool not found: %s", name)
	}

	// 验证输入参数
	if err := tool.Validate(input); err != nil {
		r.logger.Error("Parameter validation failed",
			"tool", name,
			"error", err)
		return "", &ValidationError{
			Field:   "",
			Message: fmt.Sprintf("parameter validation failed: %s", err.Error()),
		}
	}

	// 执行工具
	r.logger.Info("Executing tool",
		"tool", name,
		"input", input)
	output, err := tool.Execute(ctx, input)
	if err != nil {
		r.logger.Error("Tool execution failed",
			"tool", name,
			"error", err)
		return "", &ToolExecuteError{
			ToolName: name,
			Err:      err,
		}
	}

	return output, nil
}

// String 返回注册表的字符串表示
func (r *Registry) String() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := r.Names()
	return fmt.Sprintf("Registry(%d tools: %s)", len(names), strings.Join(names, ", "))
}
