package context

import (
	"time"

	"github.com/yourname/oxencode/pkg/config"
)

// PageType 页面类型，表示上下文层级
type PageType string

const (
	PageTypeL0 PageType = "l0" // 全局高层次压缩
	PageTypeL1 PageType = "l1" // 轻度压缩的交互轮次
	PageTypeL2 PageType = "l2" // 原始 messages
)

// PageID 页面唯一标识
type PageID string

// CompressionStrategy 压缩策略配置
type CompressionStrategy struct {
	// 压缩率限制
	MaxCompressionRate float64 `json:"max_compression_rate"` // 最大压缩率
	MinCompressionRate float64 `json:"min_compression_rate"` // 最小压缩率

	// Schema 模板
	Schema string `json:"schema"` // 压缩使用的 schema 模板

	// Skill 配置
	Skill string `json:"skill"` // 用于特定压缩任务的 skill

	// 压缩模型标识
	CompressionModel string `json:"compression_model"` // 用于压缩的模型标识

	// 超时配置
	Timeout time.Duration `json:"timeout"` // 压缩超时时间

	// 截断配置（从Config读取）
	MaxToolOutputLength int `json:"max_tool_output_length"` // 工具输出最大长度，0表示不截断
	MaxAssistantLength  int `json:"max_assistant_length"`   // Assistant消息最大长度，0表示不截断
}

// NewCompressionStrategy 从Config创建指定PageType的压缩策略
func NewCompressionStrategy(pageType PageType, cfg *config.Config) *CompressionStrategy {
	switch pageType {
	case PageTypeL1:
		return &CompressionStrategy{
			MaxCompressionRate:  0.5,
			MinCompressionRate:  0.2,
			Schema:              L1SchemaTemplate,
			Timeout:             30 * time.Second,
			MaxToolOutputLength: cfg.L1MaxToolOutputLength,
			MaxAssistantLength:  cfg.L1MaxAssistantLength,
		}
	case PageTypeL0:
		return &CompressionStrategy{
			MaxCompressionRate:  0.3,
			MinCompressionRate:  0.1,
			Schema:              L0SchemaTemplate,
			Timeout:             60 * time.Second,
			MaxToolOutputLength: 0, // 不截断，LLM处理
			MaxAssistantLength:  0,
		}
	case PageTypeL2:
		return &CompressionStrategy{
			MaxCompressionRate:  1.0,
			MinCompressionRate:  1.0,
			Schema:              "",
			Timeout:             5 * time.Second,
			MaxToolOutputLength: 0, // 不截断
			MaxAssistantLength:  0,
		}
	}
	return nil
}

// DefaultCompressionStrategies 返回默认的压缩策略配置（使用默认Config）
func DefaultCompressionStrategies() (L0, L1, L2 *CompressionStrategy) {
	cfg := config.Get()
	if cfg == nil {
		// 如果没有配置，使用硬编码默认值
		L1 = &CompressionStrategy{
			MaxCompressionRate:  0.5,
			MinCompressionRate:  0.2,
			Schema:              L1SchemaTemplate,
			Timeout:             30 * time.Second,
			MaxToolOutputLength: 1000,
			MaxAssistantLength:  2000,
		}

		L0 = &CompressionStrategy{
			MaxCompressionRate:  0.3,
			MinCompressionRate:  0.1,
			Schema:              L0SchemaTemplate,
			Timeout:             60 * time.Second,
			MaxToolOutputLength: 0,
			MaxAssistantLength:  0,
		}

		L2 = &CompressionStrategy{
			MaxCompressionRate:  1.0,
			MinCompressionRate:  1.0,
			Schema:              "",
			Timeout:             5 * time.Second,
			MaxToolOutputLength: 0,
			MaxAssistantLength:  0,
		}
		return L0, L1, L2
	}

	return NewCompressionStrategy(PageTypeL0, cfg),
		NewCompressionStrategy(PageTypeL1, cfg),
		NewCompressionStrategy(PageTypeL2, cfg)
}

// Schema templates
const (
	// L1SchemaTemplate L1 压缩 schema：对一轮交互的摘要
	L1SchemaTemplate = `# User Input
用户的原始输入是什么，用户意图是什么

# Agent Actions
Agent 执行了哪些操作（工具调用），每个操作的关键结果是什么
- 工具名称：简要描述
- 工具名称：简要描述
- Agent 输出摘要

# Task Status
当前任务的完成状态是什么（进行中/已完成/遇到问题）

# Key Information
这轮对话中的关键信息（代码片段、配置、重要发现、错误、矛盾等）
`

	// L0SchemaTemplate L0 压缩 schema：高层次概括
	L0SchemaTemplate = `# Project Context
项目背景、技术栈、项目结构等

# User Inputs
- input1
- input2
- ...

# Key Achievements
已完成的主要工作

# Key Information
关键代码位置和片段、关键发现、矛盾错误

# Core Concepts
核心概念、关键词以及简短的解释

# Important Notes
重要的约束条件、配置、代码风格等高层次信息
`
)
