package message

// Role 消息角色类型
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// Message 消息结构
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// NewMessage 创建新消息
func NewMessage(role Role, content string) Message {
	return Message{
		Role:    role,
		Content: content,
	}
}
