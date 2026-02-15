package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Model TUI 主模型
type Model struct {
	messages  []string          // 消息历史
	input     string            // 用户输入
	quitting  bool              // 是否正在退出
	err       error             // 错误状态
}

// InitialModel 创建初始模型
func InitialModel() Model {
	return Model{
		messages: []string{"Welcome to OxenCode! Type your message and press Enter."},
		input:    "",
		quitting: false,
		err:      nil,
	}
}

// Init 初始化模型
func (m Model) Init() tea.Cmd {
	return nil
}

// Update 更新模型状态
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit

		case tea.KeyEnter:
			if m.input != "" {
				// 将用户消息添加到历史
				m.messages = append(m.messages, "You: "+m.input)
				m.input = ""
			}
			return m, nil

		case tea.KeyBackspace:
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
			return m, nil

		default:
			// 添加字符到输入
			if msg.Type == tea.KeyRunes {
				m.input += string(msg.Runes)
			}
			return m, nil
		}
	}

	return m, nil
}

// View 渲染视图
func (m Model) View() string {
	// 简单的视图实现
	s := "OxenCode - AI Programming Assistant\n\n"

	// 显示消息历史
	for _, msg := range m.messages {
		s += msg + "\n"
	}

	s += "\n"

	// 显示输入框
	s += "> " + m.input

	if m.quitting {
		s += "\nGoodbye!"
	}

	return s
}
