package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yourname/oxencode/internal/message"
	"github.com/yourname/oxencode/internal/ui/components"
)

// Model 主模型
type Model struct {
	input       components.Input
	messages    components.Messages
	width       int
	height      int
	quitting    bool
	showWelcome bool
}

// InitialModel 创建初始模型
func InitialModel() Model {
	return Model{
		input: components.Input{
			Prompt: "> ",
			Focus:  true,
			Style:  InputInnerStyle,
		},
		messages:    components.NewMessages(),
		showWelcome: true,
	}
}

// Init 初始化
func (m Model) Init() tea.Cmd {
	return nil
}

// Update 更新状态
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case components.UserSubmitMsg:
		if msg.Content == "" {
			return m, nil
		}
		m.showWelcome = false
		m.messages.Update(message.UserMsg{Content: msg.Content})
		m.input.Value = ""
		m.input.Cursor = 0
	}

	var cmd tea.Cmd
	model, cmd := m.input.Update(msg)
	m.input = model.(components.Input)
	return m, cmd
}

// View 渲染视图
func (m Model) View() string {
	if m.quitting {
		return "Bye!\n"
	}

	if m.width == 0 {
		return "Loading..."
	}

	// 构建视图
	var sections []string

	// 1. 欢迎信息或消息历史
	if m.showWelcome && len(m.messages.Items) == 0 {
		sections = append(sections, GetWelcomeStyle())
	} else {
		sections = append(sections, m.messages.View())
	}

	// 2. 输入框
	sections = append(sections, m.renderInput())

	// 3. 状态栏
	sections = append(sections, StatusStyle.Width(m.width).Render("Press Ctrl+C or q to quit"))

	// 垂直排列所有部分
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderInput 渲染输入框
func (m Model) renderInput() string {
	// 直接渲染输入框，不添加光标符号
	inputLine := m.input.Prompt + m.input.Value
	return InputContainerStyle.Width(m.width).Render(inputLine)
}
