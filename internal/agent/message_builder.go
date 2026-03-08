package agent

import (
	"encoding/json"

	"charm.land/fantasy"
	ctxpkg "github.com/yourname/oxencode/internal/context"
	"github.com/yourname/oxencode/internal/message"
	"github.com/yourname/oxencode/pkg/config"
	"github.com/yourname/oxencode/pkg/logger"
)

// MessageBuilder 构建发送给 LLM 的消息
// 负责将 Session 中的消息转换为 fantasy.Message 格式
type MessageBuilder struct {
	session *ctxpkg.Session
	config  *config.Config
	logger  logger.Logger
}

// NewMessageBuilder 创建消息构建器
func NewMessageBuilder(session *ctxpkg.Session, config *config.Config, logger logger.Logger) *MessageBuilder {
	return &MessageBuilder{
		session: session,
		config:  config,
		logger:  logger,
	}
}

// Build 构建消息列表
// systemPrompt 作为系统消息添加到开头
func (b *MessageBuilder) Build(systemPrompt string) []fantasy.Message {
	// 从 Session 获取上下文
	ctxMsgs := b.session.GetContext()

	// 转换为 fantasy.Message
	messages := make([]fantasy.Message, 0, len(ctxMsgs)+1)

	// 添加系统提示词（如果提供）
	if systemPrompt != "" {
		messages = append(messages, fantasy.NewSystemMessage(systemPrompt))
	}

	// 转换每条消息
	for _, msg := range ctxMsgs {
		// 跳过系统消息（已经在上面添加了）
		if msg.Role == message.RoleSystem {
			continue
		}
		messages = append(messages, b.convertMessage(msg))
	}

	b.logger.Debug("Messages built", "count", len(messages))
	return messages
}

// convertMessage 将内部消息转换为 fantasy.Message
func (b *MessageBuilder) convertMessage(msg message.Message) fantasy.Message {
	switch msg.Role {
	case message.RoleSystem:
		return fantasy.NewSystemMessage(msg.Content)

	case message.RoleUser:
		return fantasy.NewUserMessage(msg.Content)

	case message.RoleAssistant:
		// Assistant 消息可能包含工具调用
		parts := []fantasy.MessagePart{
			fantasy.TextPart{Text: msg.Content},
		}

		// 添加工具调用（如果有）
		for _, step := range msg.ReActLoop {
			if step.ToolCall != nil {
				inputJSON, _ := json.Marshal(step.ToolCall.Input)
				parts = append(parts, fantasy.ToolCallPart{
					ToolCallID: step.ToolCall.ID,
					ToolName:   step.ToolCall.Name,
					Input:      string(inputJSON),
				})
			}
		}

		return fantasy.Message{
			Role:    fantasy.MessageRoleAssistant,
			Content: parts,
		}

	case message.RoleTool:
		// 工具结果作为用户消息的一部分
		// 注意：fantasy 的工具结果格式需要特殊处理
		return fantasy.Message{
			Role: fantasy.MessageRoleUser,
			Content: []fantasy.MessagePart{
				fantasy.TextPart{Text: "[Tool Result]\n" + msg.Content},
			},
		}

	default:
		// 未知角色，作为用户消息处理
		return fantasy.NewUserMessage(msg.Content)
	}
}

// BuildWithTools 构建包含工具定义的消息
// 用于支持工具调用的场景
func (b *MessageBuilder) BuildWithTools(systemPrompt string, tools []fantasy.Tool) []fantasy.Message {
	messages := b.Build(systemPrompt)
	// 工具定义在 Call 结构中传递，不需要在消息中包含
	return messages
}