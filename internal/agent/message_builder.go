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
func (b *MessageBuilder) Build() []fantasy.Message {
	// 从 Session 获取上下文
	ctxMsgs := b.session.GetContext()

	// 转换为 fantasy.Message
	messages := make([]fantasy.Message, 0, len(ctxMsgs)+1)

	// 转换每条消息
	for _, msg := range ctxMsgs {
		converted := b.convertMessage(msg)
		messages = append(messages, converted)
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
				b.logger.Info("Adding tool call to assistant message", "tool_call_id", step.ToolCall.ID, "tool_name", step.ToolCall.Name)
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
		// 工具结果需要关联到对应的 tool_call_id
		if msg.ToolCallID != "" {
			b.logger.Info("Converting tool result", "tool_call_id", msg.ToolCallID, "content_len", len(msg.Content))
			// 使用正确的 ToolResultPart 格式，role 应该是 MessageRoleTool
			return fantasy.Message{
				Role: fantasy.MessageRoleTool,
				Content: []fantasy.MessagePart{
					fantasy.ToolResultPart{
						ToolCallID: msg.ToolCallID,
						Output:     fantasy.ToolResultOutputContentText{Text: msg.Content},
					},
				},
			}
		}
		b.logger.Warn("Tool result without tool_call_id", "content_len", len(msg.Content))
		// 如果没有 tool_call_id，使用旧格式（兼容）
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
