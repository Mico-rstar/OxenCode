package message

import (
	"time"

	"github.com/google/uuid"
)

// Role 消息角色类型
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// Status 消息状态
type Status string

const (
	StatusPending    Status = "pending"     // 等待中
	StatusStreaming  Status = "streaming"   // 流式输出中
	StatusCompleted  Status = "completed"   // 已完成
	StatusError      Status = "error"       // 出错
	StatusCancelled  Status = "cancelled"   // 已取消
)

// ToolCall 工具调用信息
type ToolCall struct {
	ID      string            `json:"id"`                // 工具调用唯一ID
	Name    string            `json:"name"`              // 工具名称
	Input   map[string]any    `json:"input"`             // 工具输入参数
	Output  string            `json:"output,omitempty"`  // 工具输出结果
	Status  Status            `json:"status"`            // 执行状态
	Error   string            `json:"error,omitempty"`   // 错误信息
}

// ReActStep ReAct循环的一步
type ReActStep struct {
	Type      string    `json:"type"`       // "thought", "action", "observation"
	Content   string    `json:"content"`    // 内容
	Timestamp time.Time `json:"timestamp"`  // 时间戳
	ToolCall  *ToolCall `json:"tool,omitempty"` // 工具调用信息
}

// Message 消息结构
type Message struct {
	ID          string      `json:"id"`                    // 消息唯一ID
	Role        Role        `json:"role"`                  // 消息角色
	Content     string      `json:"content"`               // 消息内容
	Status      Status      `json:"status"`                // 消息状态
	Timestamp   time.Time   `json:"timestamp"`             // 时间戳
	ReActLoop   []ReActStep `json:"react_loop,omitempty"`  // ReAct循环步骤
	IsStreaming bool        `json:"is_streaming"`          // 是否正在流式输出
	ToolCallID  string      `json:"tool_call_id,omitempty"` // 工具调用ID（仅对 RoleTool 有效）
}

// NewMessage 创建新消息
func NewMessage(role Role, content string) Message {
	return Message{
		ID:        GenerateID(),
		Role:      role,
		Content:   content,
		Status:    StatusCompleted,
		Timestamp: time.Now(),
		ReActLoop: []ReActStep{},
	}
}

// NewToolResultMessage 创建工具结果消息
func NewToolResultMessage(toolCallID, content string) Message {
	return Message{
		ID:         GenerateID(),
		Role:       RoleTool,
		Content:    content,
		Status:     StatusCompleted,
		Timestamp:  time.Now(),
		ReActLoop:  []ReActStep{},
		ToolCallID: toolCallID,
	}
}

// NewStreamingMessage 创建流式消息
func NewStreamingMessage(role Role) Message {
	return Message{
		ID:        GenerateID(),
		Role:      role,
		Content:   "",
		Status:    StatusStreaming,
		Timestamp: time.Now(),
		ReActLoop: []ReActStep{},
		IsStreaming: true,
	}
}

// AppendContent 追加内容（用于流式输出）
func (m *Message) AppendContent(delta string) {
	m.Content += delta
}

// AddReActStep 添加ReAct步骤
func (m *Message) AddReActStep(stepType, content string) {
	step := ReActStep{
		Type:      stepType,
		Content:   content,
		Timestamp: time.Now(),
	}
	m.ReActLoop = append(m.ReActLoop, step)
}

// AppendReActStep 累积ReAct步骤内容（用于流式reasoning内容）
// 如果最后一个步骤类型相同，则追加内容；否则添加新步骤
func (m *Message) AppendReActStep(stepType, content string) {
	// 检查最后一个步骤是否是相同类型
	if len(m.ReActLoop) > 0 && m.ReActLoop[len(m.ReActLoop)-1].Type == stepType {
		// 累积内容到最后一个步骤
		m.ReActLoop[len(m.ReActLoop)-1].Content += content
	} else {
		// 添加新步骤
		m.AddReActStep(stepType, content)
	}
}

// AddToolCall 添加工具调用步骤
func (m *Message) AddToolCall(toolName string, input map[string]any) string {
	// 生成唯一的工具调用ID (UUID)
	toolCallID := uuid.New().String()
	return m.AddToolCallWithID(toolCallID, toolName, input)
}

// AddToolCallWithID 添加工具调用步骤（使用指定的ID）
func (m *Message) AddToolCallWithID(toolCallID, toolName string, input map[string]any) string {
	toolCall := &ToolCall{
		ID:     toolCallID,
		Name:   toolName,
		Input:  input,
		Status: StatusPending,
	}

	step := ReActStep{
		Type:      "action",
		Content:   "",
		Timestamp: time.Now(),
		ToolCall:  toolCall,
	}

	m.ReActLoop = append(m.ReActLoop, step)
	return toolCallID // 返回工具调用ID
}

// UpdateToolCall 更新工具调用结果
// 查找最后一个匹配工具名称且处于 pending 状态的工具调用进行更新
func (m *Message) UpdateToolCall(toolName string, output string, status Status, errMsg string) {
	// 从后往前查找，找到最后一个匹配且pending的工具调用
	for i := len(m.ReActLoop) - 1; i >= 0; i-- {
		if m.ReActLoop[i].ToolCall != nil &&
			m.ReActLoop[i].ToolCall.Name == toolName &&
			m.ReActLoop[i].ToolCall.Status == StatusPending {
			m.ReActLoop[i].ToolCall.Output = output
			m.ReActLoop[i].ToolCall.Status = status
			m.ReActLoop[i].ToolCall.Error = errMsg
			if status == StatusError {
				m.ReActLoop[i].Content = errMsg
			} else {
				m.ReActLoop[i].Content = output
			}
			break
		}
	}
}

// Complete 完成消息
func (m *Message) Complete() {
	m.Status = StatusCompleted
	m.IsStreaming = false
}

// Cancel 取消消息
func (m *Message) Cancel() {
	m.Status = StatusCancelled
	m.IsStreaming = false
}

// SetError 设置错误状态
func (m *Message) SetError(err error) {
	m.Status = StatusError
	m.IsStreaming = false
	if err != nil {
		m.Content = err.Error()
	}
}

// GenerateID 生成唯一ID (使用UUID)
func GenerateID() string {
	return uuid.New().String()
}

// ============================================
// AtomSequence - 原子消息序列
// ============================================

// AtomSequence 原子消息序列
// 表示一个不可分割的消息单元：一个 assistant 消息 + 多个 tool results
// 设计原则：在 Page 构建之初就确保原子性，避免 orphan tool messages 问题
type AtomSequence struct {
	ID          string    `json:"id"`          // 序列唯一ID
	Assistant   *Message  `json:"assistant"`   // assistant 消息（必须存在）
	ToolResults []Message `json:"tool_results"` // tool result 消息列表（可为空）
	UserMessage *Message  `json:"user_message"` // 可选的 user 消息（原子序列开头的 user）
	CreatedAt   time.Time `json:"created_at"`
	Completed   bool      `json:"completed"` // 序列是否已完成（收到最终响应）
}

// NewAtomSequence 创建新的原子序列
func NewAtomSequence() *AtomSequence {
	return &AtomSequence{
		ID:          GenerateID(),
		ToolResults: make([]Message, 0),
		CreatedAt:   time.Now(),
		Completed:   false,
	}
}

// SetAssistant 设置 assistant 消息
func (a *AtomSequence) SetAssistant(msg Message) {
	a.Assistant = &msg
}

// SetUserMessage 设置 user 消息（可选，在 assistant 之前）
func (a *AtomSequence) SetUserMessage(msg Message) {
	a.UserMessage = &msg
}

// AddToolResult 添加 tool result
func (a *AtomSequence) AddToolResult(msg Message) {
	a.ToolResults = append(a.ToolResults, msg)
}

// HasToolCalls 检查 assistant 消息是否有 tool calls
func (a *AtomSequence) HasToolCalls() bool {
	if a.Assistant == nil {
		return false
	}
	for _, step := range a.Assistant.ReActLoop {
		if step.ToolCall != nil {
			return true
		}
	}
	return false
}

// IsComplete 检查序列是否完整
// 完整 = assistant 的所有 tool_calls 都有对应的 tool result
func (a *AtomSequence) IsComplete() bool {
	if a.Assistant == nil {
		return false
	}

	// 收集所有 tool_call_ids
	expectedIDs := make(map[string]bool)
	for _, step := range a.Assistant.ReActLoop {
		if step.ToolCall != nil {
			expectedIDs[step.ToolCall.ID] = true
		}
	}

	// 如果没有 tool calls，则 assistant 消息本身就是完整的
	if len(expectedIDs) == 0 {
		return true
	}

	// 检查是否都有对应的 tool result
	for _, result := range a.ToolResults {
		if result.ToolCallID != "" {
			delete(expectedIDs, result.ToolCallID)
		}
	}

	return len(expectedIDs) == 0
}

// ToMessages 转换为消息列表
// 顺序：[user_message (可选)] + [assistant] + [tool_results...]
func (a *AtomSequence) ToMessages() []Message {
	msgs := make([]Message, 0)
	if a.UserMessage != nil {
		msgs = append(msgs, *a.UserMessage)
	}
	if a.Assistant != nil {
		msgs = append(msgs, *a.Assistant)
	}
	msgs = append(msgs, a.ToolResults...)
	return msgs
}

// GetTokenCount 估算 token 数
func (a *AtomSequence) GetTokenCount() int {
	total := 0
	for _, msg := range a.ToMessages() {
		total += len(msg.Content) / 4
	}
	return total
}

// GetToolCallIDs 获取该原子序列中所有的 tool_call_ids
func (a *AtomSequence) GetToolCallIDs() []string {
	ids := make([]string, 0)
	if a.Assistant != nil {
		for _, step := range a.Assistant.ReActLoop {
			if step.ToolCall != nil {
				ids = append(ids, step.ToolCall.ID)
			}
		}
	}
	return ids
}
