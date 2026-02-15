package message

// Message 消息接口
type Message interface {
	IsMessage()
}

// UserMsg 用户输入消息
type UserMsg struct {
	Content string
}

func (UserMsg) IsMessage() {}

// AIMsg AI 响应消息（预留）
type AIMsg struct {
	Content string
}

func (AIMsg) IsMessage() {}

// SystemMsg 系统消息（预留）
type SystemMsg struct {
	Content string
}

func (SystemMsg) IsMessage() {}
