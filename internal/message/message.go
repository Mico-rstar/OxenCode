package message

import (
	"time"
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
	ID        string      `json:"id"`                  // 消息唯一ID
	Role      Role        `json:"role"`                // 消息角色
	Content   string      `json:"content"`             // 消息内容
	Status    Status      `json:"status"`              // 消息状态
	Timestamp time.Time   `json:"timestamp"`           // 时间戳
	ReActLoop []ReActStep `json:"react_loop,omitempty"` // ReAct循环步骤
	IsStreaming bool      `json:"is_streaming"`        // 是否正在流式输出
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

// AddToolCall 添加工具调用步骤
func (m *Message) AddToolCall(toolName string, input map[string]any) string {
	toolCall := &ToolCall{
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
	return toolCall.Name // 返回工具调用ID
}

// UpdateToolCall 更新工具调用结果
func (m *Message) UpdateToolCall(toolName string, output string, status Status, errMsg string) {
	for i := range m.ReActLoop {
		if m.ReActLoop[i].ToolCall != nil && m.ReActLoop[i].ToolCall.Name == toolName {
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

// GenerateID 生成唯一ID
func GenerateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(6)
}

// randomString 生成随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond) // 确保随机性
	}
	return string(b)
}
