package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yourname/oxencode/internal/message"
)

// 消息样式定义
var (
	userMsgStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("42")).
		Bold(true)

	aiMsgStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("226"))
)

// Messages 消息显示组件
type Messages struct {
	Items []message.Message
	Width int
	Style lipgloss.Style
}

// NewMessages 创建消息组件
func NewMessages() Messages {
	return Messages{
		Items: []message.Message{},
	}
}

// Init 初始化
func (m Messages) Init() tea.Cmd {
	return nil
}

// Update 更新状态
func (m Messages) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case message.UserMsg:
		m.Items = append(m.Items, msg)
	case message.AIMsg:
		m.Items = append(m.Items, msg)
	}
	return m, nil
}

// View 渲染视图
func (m Messages) View() string {
	var s string
	for _, item := range m.Items {
		s += m.renderMessage(item) + "\n\n"
	}
	// 不显示占位符文本
	return m.Style.Width(m.Width).Render(s)
}

// renderMessage 渲染单条消息
func (m Messages) renderMessage(msg message.Message) string {
	switch v := msg.(type) {
	case message.UserMsg:
		return userMsgStyle.Render("You: " + v.Content)
	case message.AIMsg:
		return aiMsgStyle.Render("AI: " + v.Content)
	default:
		return ""
	}
}
