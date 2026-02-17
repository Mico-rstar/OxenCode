package tools

import (
	"encoding/json"
	"fmt"

	"github.com/yourname/oxencode/pkg/logger"
)

// Validator 参数验证器
// 使用 JSON Schema 验证工具输入参数
type Validator struct {
	schema json.RawMessage
	logger logger.Logger
}

// NewValidator 创建参数验证器
// logger 是可选的，如果传入 nil 则创建基于全局 logger 的实例
func NewValidator(schema json.RawMessage, log logger.Logger) *Validator {
	if log == nil {
		log = logger.New("validator")
	}
	return &Validator{
		schema: schema,
		logger: log,
	}
}

// Validate 验证输入参数
func (v *Validator) Validate(input map[string]any) error {
	// 解析 schema
	var schema struct {
		Type       string                     `json:"type"`
		Properties map[string]PropertySchema `json:"properties"`
		Required   []string                   `json:"required"`
	}

	if err := json.Unmarshal(v.schema, &schema); err != nil {
		v.logger.Warn("Failed to parse schema", "error", err)
		return nil // 如果无法解析 schema，跳过验证
	}

	// 检查必填字段
	for _, field := range schema.Required {
		if _, exists := input[field]; !exists {
			v.logger.Warn("Missing required field", "field", field)
			return &ValidationError{
				Field:   field,
				Message: fmt.Sprintf("required field '%s' is missing", field),
			}
		}
	}

	// 验证每个输入字段的类型
	for key, value := range input {
		propSchema, exists := schema.Properties[key]
		if !exists {
			v.logger.Warn("Unknown field", "field", key)
			continue // 未知字段，跳过
		}

		if err := v.validateValue(key, value, propSchema); err != nil {
			return err
		}
	}

	v.logger.Debug("Validation passed")
	return nil
}

// PropertySchema 属性 schema
type PropertySchema struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

// validateValue 验证单个值
func (v *Validator) validateValue(field string, value any, schema PropertySchema) error {
	// 检查枚举值
	if len(schema.Enum) > 0 {
		strValue, ok := value.(string)
		if !ok {
			return &ValidationError{
				Field:   field,
				Message: fmt.Sprintf("field '%s' must be a string for enum validation", field),
			}
		}

		valid := false
		for _, enum := range schema.Enum {
			if strValue == enum {
				valid = true
				break
			}
		}

		if !valid {
			return &ValidationError{
				Field:   field,
				Message: fmt.Sprintf("field '%s' must be one of: %v", field, schema.Enum),
			}
		}
	}

	// 检查类型
	switch schema.Type {
	case "string":
		if _, ok := value.(string); !ok {
			return &ValidationError{
				Field:   field,
				Message: fmt.Sprintf("field '%s' must be a string", field),
			}
		}

	case "integer", "number":
		// accept float64 or int
		switch value.(type) {
		case float64, int, int64, json.Number:
			// valid
		default:
			return &ValidationError{
				Field:   field,
				Message: fmt.Sprintf("field '%s' must be a number", field),
			}
		}

	case "boolean":
		if _, ok := value.(bool); !ok {
			return &ValidationError{
				Field:   field,
				Message: fmt.Sprintf("field '%s' must be a boolean", field),
			}
		}

	case "array":
		if _, ok := value.([]any); !ok {
			return &ValidationError{
				Field:   field,
				Message: fmt.Sprintf("field '%s' must be an array", field),
			}
		}

	case "object":
		if _, ok := value.(map[string]any); !ok {
			return &ValidationError{
				Field:   field,
				Message: fmt.Sprintf("field '%s' must be an object", field),
			}
		}

	default:
		v.logger.Warn("Unknown type in schema",
			"field", field,
			"type", schema.Type)
	}

	return nil
}

// ValidateInput 便捷函数：使用给定的 schema 验证输入
func ValidateInput(schema json.RawMessage, input map[string]any) error {
	validator := NewValidator(schema, nil)
	return validator.Validate(input)
}
