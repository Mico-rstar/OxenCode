package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/yourname/oxencode/internal/message"
)

// renderHeader 渲染状态栏
func (m Model) renderHeader() string {
	leftPart := m.styles.Header.Render("OxenCode - AI Programming Assistant")
	middlePart := m.styles.Header.Render(m.statusModel)
	rightPart := m.styles.Header.Render(m.statusConn + " " + m.statusTime)

	header := lipgloss.JoinHorizontal(lipgloss.Top, leftPart, middlePart, rightPart)
	return lipgloss.NewStyle().Width(m.width).Render(header)
}

// renderMessages 渲染消息区域
func (m Model) renderMessages() string {
	var b strings.Builder

	// 计算可用高度
	headerHeight := 1
	footerHeight := 3
	availableHeight := m.height - headerHeight - footerHeight - 2

	for _, msg := range m.messages {
		b.WriteString(m.renderMessage(msg))
		b.WriteString("\n\n")
	}

	// 截断以适应窗口
	msgContent := b.String()
	lines := strings.Split(msgContent, "\n")
	if len(lines) > availableHeight {
		lines = lines[len(lines)-availableHeight:]
	}

	return m.styles.MessageArea.Render(strings.Join(lines, "\n"))
}

// renderMessage 渲染单条消息
func (m Model) renderMessage(msg message.Message) string {
	icon := GetIconForRole(msg.Role)
	statusIcon := GetStatusIcon(msg.Status)
	timestamp := msg.Timestamp.Format("15:04:05")

	var b strings.Builder

	// 消息头部
	switch msg.Role {
	case message.RoleUser:
		header := lipgloss.JoinHorizontal(
			lipgloss.Top,
			m.styles.IconUser.Render(icon),
			" User ",
			m.styles.Muted.Render(timestamp),
		)
		b.WriteString(m.styles.UserMsg.Render(header))
		b.WriteString("\n")
		b.WriteString(msg.Content)

	case message.RoleAssistant:
		header := lipgloss.JoinHorizontal(
			lipgloss.Top,
			m.styles.IconAssistant.Render(icon),
			" Assistant ",
			m.styles.Muted.Render(statusIcon),
		)
		b.WriteString(m.styles.AssistantMsg.Render(header))

		// 渲染 ReAct 循环
		if len(msg.ReActLoop) > 0 {
			b.WriteString("\n")
			for i, step := range msg.ReActLoop {
				b.WriteString(m.renderReActStep(step, i == len(msg.ReActLoop)-1))
			}
		}

		if msg.Content != "" {
			b.WriteString("\n")
			b.WriteString(msg.Content)
		}

	case message.RoleSystem:
		b.WriteString(m.styles.SystemMsg.Render(msg.Content))
	}

	return b.String()
}

// renderReActStep 渲染 ReAct 步骤
func (m Model) renderReActStep(step message.ReActStep, isLast bool) string {
	var prefix string
	if isLast {
		prefix = "└─"
	} else {
		prefix = "├─"
	}

	var content string
	switch step.Type {
	case "thought":
		content = lipgloss.JoinHorizontal(
			lipgloss.Top,
			m.styles.IconThought.Render(IconThought),
			" "+step.Content,
		)
	case "action":
		if step.ToolCall != nil {
			statusIcon := GetStatusIcon(step.ToolCall.Status)
			content = lipgloss.JoinHorizontal(
				lipgloss.Top,
				m.styles.IconTool.Render(IconTool),
				" "+step.ToolCall.Name,
				" "+m.styles.Muted.Render(statusIcon),
			)
			if step.ToolCall.Output != "" {
				content += "\n" + m.styles.Muted.Render("  ├─ Result: "+step.ToolCall.Output)
			}
		} else {
			content = step.Content
		}
	default:
		content = step.Content
	}

	return m.styles.ReActBranch.Render(prefix + " " + content)
}

// renderFooter 渲染输入区域
func (m Model) renderFooter() string {
	// 输入框
	prompt := m.styles.InputPrompt.Render("> ")
	input := m.input
	if m.cursor < len(input) {
		input = input[:m.cursor] + "│" + input[m.cursor:]
	} else {
		input = input + "│"
	}

	inputLine := lipgloss.JoinHorizontal(lipgloss.Top, prompt, input)

	// 帮助文本
	help := m.styles.InputHelp.Render("[Enter: Send]  [Esc: Cancel]  [Ctrl+C: Quit]")

	return lipgloss.JoinVertical(lipgloss.Left, inputLine, help)
}

// renderPermissionModal 渲染权限确认弹窗
func (m Model) renderPermissionModal() string {
	title := m.styles.ModalTitle.Render(IconWarning + " Permission Required")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		m.styles.Bold.Render("Tool: "+m.permission.ToolName),
		m.styles.Bold.Render("Operation: "+m.permission.Operation),
		"",
		m.permission.Desc,
		"",
	)

	// 按钮区域
	var buttons []string
	for i, choice := range m.permission.Choices {
		if i == m.permission.Cursor {
			buttons = append(buttons, m.styles.ModalButtonFocus.Render("["+choice+"]"))
		} else {
			buttons = append(buttons, m.styles.ModalButton.Render(" "+choice+" "))
		}
	}
	buttonRow := lipgloss.JoinHorizontal(lipgloss.Left, buttons...)

	footer := lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		buttonRow,
		"",
		m.styles.Muted.Render("Press Enter to select, Esc to cancel"),
	)

	modalContent := lipgloss.JoinVertical(lipgloss.Left, title, content, footer)

	// 居中弹窗
	modal := m.styles.Modal.Render(modalContent)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		modal,
	)
}
