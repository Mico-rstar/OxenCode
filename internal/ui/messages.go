package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yourname/oxencode/internal/message"
)

// SendMessage 用户发送消息的命令
type SendMessage struct {
	Content string
}

// SentMessageSent 消息发送完成的消息
type SentMessageSent struct{}

// StreamStartMsg 开始流式输出的消息
type StreamStartMsg struct {
	MessageID    string
	UserContent  string // 用户消息内容
}

// StreamDeltaMsg 流式输出增量消息
type StreamDeltaMsg struct {
	MessageID string
	Delta     string
}

// StreamCompleteMsg 流式输出完成消息
type StreamCompleteMsg struct {
	MessageID string
}

// StreamErrorMsg 流式输出错误消息
type StreamErrorMsg struct {
	MessageID string
	Error     error
}

// ReActStepMsg ReAct步骤消息
type ReActStepMsg struct {
	MessageID string
	StepType  string
	Content   string
}

// ToolCallMsg 工具调用消息
type ToolCallMsg struct {
	MessageID string
	ToolName  string
	Input     map[string]any
}

// ToolResultMsg 工具执行结果消息
type ToolResultMsg struct {
	MessageID string
	ToolName  string
	Output    string
	Error     string
	Success   bool
}

// PermissionRequestMsg 权限请求消息
type PermissionRequestMsg struct {
	MessageID string
	ToolName  string
	Operation string
	Desc      string
}

// PermissionResponseMsg 权限响应消息
type PermissionResponseMsg struct {
	Allowed bool
	Always  bool // 是否永久授权
}

// InterruptMsg 中断消息（用户按Esc）
type InterruptMsg struct{}

// StatusTickMsg 状态栏定时更新消息
type StatusTickMsg struct {
	Time string
}

// AgentTickMsg Agent处理定时检查消息
type AgentTickMsg struct{}

// NewUserMessage 创建用户消息命令
func NewUserMessage(content string) tea.Cmd {
	return func() tea.Msg {
		return SendMessage{Content: content}
	}
}

// StreamStart 返回流式开始消息命令
func StreamStart(msgID string) tea.Cmd {
	return func() tea.Msg {
		return StreamStartMsg{MessageID: msgID}
	}
}

// StreamDelta 返回流式增量消息命令
func StreamDelta(msgID, delta string) tea.Cmd {
	return func() tea.Msg {
		return StreamDeltaMsg{MessageID: msgID, Delta: delta}
	}
}

// StreamComplete 返回流式完成消息命令
func StreamComplete(msgID string) tea.Cmd {
	return func() tea.Msg {
		return StreamCompleteMsg{MessageID: msgID}
	}
}

// StreamError 返回流式错误消息命令
func StreamError(msgID string, err error) tea.Cmd {
	return func() tea.Msg {
		return StreamErrorMsg{MessageID: msgID, Error: err}
	}
}

// NewReActStep 返回ReAct步骤消息命令
func NewReActStep(msgID, stepType, content string) tea.Cmd {
	return func() tea.Msg {
		return ReActStepMsg{MessageID: msgID, StepType: stepType, Content: content}
	}
}

// NewToolCall 返回工具调用消息命令
func NewToolCall(msgID, toolName string, input map[string]any) tea.Cmd {
	return func() tea.Msg {
		return ToolCallMsg{MessageID: msgID, ToolName: toolName, Input: input}
	}
}

// NewToolResult 返回工具结果消息命令
func NewToolResult(msgID, toolName, output, errMsg string, success bool) tea.Cmd {
	return func() tea.Msg {
		return ToolResultMsg{MessageID: msgID, ToolName: toolName, Output: output, Error: errMsg, Success: success}
	}
}

// RequestPermission 返回权限请求命令
func RequestPermission(msgID, toolName, operation, desc string) tea.Cmd {
	return func() tea.Msg {
		return PermissionRequestMsg{MessageID: msgID, ToolName: toolName, Operation: operation, Desc: desc}
	}
}

// SendPermissionResponse 返回权限响应命令
func SendPermissionResponse(allowed, always bool) tea.Cmd {
	return func() tea.Msg {
		return PermissionResponseMsg{Allowed: allowed, Always: always}
	}
}

// Tick 每秒触发一次，用于更新状态栏时间
func Tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return StatusTickMsg{Time: t.Format("15:04:05")}
	})
}

// AgentTick 用于检查Agent状态的定时器
func AgentTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return AgentTickMsg{}
	})
}

// Interrupt 发送中断信号
func Interrupt() tea.Cmd {
	return func() tea.Msg {
		return InterruptMsg{}
	}
}

// SentMessageSent 返回消息发送完成命令
func MessageSent() tea.Cmd {
	return func() tea.Msg {
		return SentMessageSent{}
	}
}

// 辅助方法：根据消息角色获取图标
func GetIconForRole(role message.Role) string {
	switch role {
	case message.RoleUser:
		return IconUser
	case message.RoleAssistant:
		return IconAssistant
	case message.RoleSystem:
		return IconSystem
	case message.RoleTool:
		return IconTool
	default:
		return "•"
	}
}

// 辅助方法：根据消息状态获取状态图标
func GetStatusIcon(status message.Status) string {
	switch status {
	case message.StatusPending:
		return "○"
	case message.StatusStreaming:
		return IconLoading
	case message.StatusCompleted:
		return IconSuccess
	case message.StatusError:
		return IconError
	case message.StatusCancelled:
		return "⊘"
	default:
		return "•"
	}
}
