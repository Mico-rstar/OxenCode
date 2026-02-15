package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Input 输入框组件
type Input struct {
	Prompt string
	Value  string
	Cursor int
	Focus  bool
	Style  lipgloss.Style
}

// Init 初始化
func (i Input) Init() tea.Cmd {
	return nil
}

// Update 更新状态
func (i Input) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			return i, func() tea.Msg {
				return UserSubmitMsg{Content: i.Value}
			}
		case tea.KeyBackspace:
			if len(i.Value) > 0 && i.Cursor > 0 {
				i.Value = i.Value[:i.Cursor-1] + i.Value[i.Cursor:]
				i.Cursor--
			}
		case tea.KeyLeft:
			if i.Cursor > 0 {
				i.Cursor--
			}
		case tea.KeyRight:
			if i.Cursor < len(i.Value) {
				i.Cursor++
			}
		case tea.KeyRunes:
			for _, r := range msg.Runes {
				i.Value = i.Value[:i.Cursor] + string(r) + i.Value[i.Cursor:]
				i.Cursor++
			}
		}
	}
	return i, nil
}

// View 渲染视图
func (i Input) View() string {
	return i.Style.Render(i.Prompt + i.Value)
}

// UserSubmitMsg 用户提交消息
type UserSubmitMsg struct {
	Content string
}
