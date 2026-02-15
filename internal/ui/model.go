package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yourname/oxencode/internal/message"
)

// AppState 应用状态
type AppState int

const (
	StateIdle AppState = iota
	StateThinking
	StateStreaming
	StateExecutingTool
	StateWaitingPermission
	StateError
)

// PermissionState 权限确认状态
type PermissionState struct {
	Active     bool
	MessageID  string
	ToolName   string
	Operation  string
	Desc       string
	Cursor     int // 0=Allow, 1=Always, 2=Deny
	Choices    []string
}

// Model TUI 主模型
type Model struct {
	// 基础状态
	messages    []message.Message // 消息历史
	input       string            // 用户输入
	cursor      int               // 光标位置
	quitting    bool              // 是否正在退出
	err         error             // 错误状态
	appState    AppState          // 应用状态

	// 尺寸
	width       int
	height      int

	// 样式
	styles      *Styles

	// 权限确认
	permission  PermissionState

	// Agent相关
	currentMsgID string // 当前正在处理的消息ID

	// 状态栏
	statusTime  string
	statusConn  string
	statusModel string
}

// NewModel 创建新模型
func NewModel() Model {
	styles := DefaultStyles()
	return Model{
		messages: []message.Message{
			message.NewMessage(
				message.RoleSystem,
				"Welcome to OxenCode! Type your message and press Enter to start.\n\nPress Esc at any time to cancel.",
			),
		},
		input:      "",
		cursor:     0,
		quitting:   false,
		err:        nil,
		appState:   StateIdle,
		width:      80,
		height:     24,
		styles:     styles,
		permission: PermissionState{
			Active:  false,
			Choices: []string{"Allow once", "Allow always", "Deny"},
		},
		statusTime: time.Now().Format("15:04:05"),
		statusConn: StatusOnline,
		statusModel: "claude-sonnet-4.5",
	}
}

// Init 初始化模型
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		Tick(),        // 启动状态栏时间更新
		// AgentTick(), // TODO: 启动Agent状态检查
	)
}

// Update 更新模型状态
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case StatusTickMsg:
		m.statusTime = msg.Time
		return m, Tick()

	case SendMessage:
		return m.handleSendMessage(msg)

	case StreamStartMsg:
		return m.handleStreamStart(msg)

	case StreamDeltaMsg:
		return m.handleStreamDelta(msg)

	case StreamCompleteMsg:
		return m.handleStreamComplete(msg)

	case StreamErrorMsg:
		return m.handleStreamError(msg)

	case ReActStepMsg:
		return m.handleReActStep(msg)

	case ToolCallMsg:
		return m.handleToolCall(msg)

	case ToolResultMsg:
		return m.handleToolResult(msg)

	case PermissionRequestMsg:
		return m.handlePermissionRequest(msg)

	case PermissionResponseMsg:
		return m.handlePermissionResponse(msg)

	case InterruptMsg:
		return m.handleInterrupt()
	}

	return m, tea.Batch(cmds...)
}

// View 渲染视图
func (m Model) View() string {
	if m.width <= 0 {
		m.width = 80
	}
	if m.height <= 0 {
		m.height = 24
	}

	// 构建各部分
	header := m.renderHeader()
	body := m.renderMessages()
	footer := m.renderFooter()

	// 如果有权限弹窗，渲染弹窗
	if m.permission.Active {
		modal := m.renderPermissionModal()
		return lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			body,
			footer,
			modal,
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		body,
		footer,
	)
}

// handleKeyMsg 处理键盘消息
func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// 如果权限弹窗激活，特殊处理
	if m.permission.Active {
		return m.handlePermissionKeyMsg(msg)
	}

	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		if m.appState == StateIdle || m.input == "" {
			m.quitting = true
			return m, tea.Quit
		}
		// 中断当前操作
		return m, Interrupt()

	case tea.KeyEnter:
		if m.input != "" && m.appState == StateIdle {
			return m, NewUserMessage(m.input)
		}
		return m, nil

	case tea.KeyBackspace:
		if len(m.input) > 0 && m.cursor > 0 {
			m.input = m.input[:m.cursor-1] + m.input[m.cursor:]
			m.cursor--
		}
		return m, nil

	case tea.KeyLeft:
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case tea.KeyRight:
		if m.cursor < len(m.input) {
			m.cursor++
		}
		return m, nil

	case tea.KeyHome:
		m.cursor = 0
		return m, nil

	case tea.KeyEnd:
		m.cursor = len(m.input)
		return m, nil

	default:
		if msg.Type == tea.KeyRunes {
			runes := string(msg.Runes)
			m.input = m.input[:m.cursor] + runes + m.input[m.cursor:]
			m.cursor += len(runes)
		}
		return m, nil
	}
}

// handlePermissionKeyMsg 处理权限弹窗的键盘消息
func (m Model) handlePermissionKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		m.permission.Active = false
		return m, SendPermissionResponse(false, false)

	case tea.KeyEnter:
		choice := m.permission.Cursor
		allowed := choice == 0 || choice == 1
		always := choice == 1
		m.permission.Active = false
		return m, SendPermissionResponse(allowed, always)

	case tea.KeyLeft, tea.KeyRight:
		m.permission.Cursor = (m.permission.Cursor + 1) % 3
		return m, nil

	case tea.KeyTab:
		m.permission.Cursor = (m.permission.Cursor + 1) % 3
		return m, nil
	}

	return m, nil
}

// handleSendMessage 处理发送消息
func (m Model) handleSendMessage(msg SendMessage) (tea.Model, tea.Cmd) {
	userMsg := message.NewMessage(message.RoleUser, msg.Content)
	m.messages = append(m.messages, userMsg)
	m.input = ""
	m.cursor = 0
	m.appState = StateThinking

	// TODO: 启动Agent处理
	// 这里暂时返回一个模拟的AI响应
	return m, func() tea.Msg {
		time.Sleep(500 * time.Millisecond)
		return StreamStartMsg{MessageID: message.GenerateID()}
	}
}

// handleStreamStart 处理流式开始
func (m Model) handleStreamStart(msg StreamStartMsg) (tea.Model, tea.Cmd) {
	aiMsg := message.NewStreamingMessage(message.RoleAssistant)
	aiMsg.ID = msg.MessageID
	m.messages = append(m.messages, aiMsg)
	m.currentMsgID = msg.MessageID
	m.appState = StateStreaming

	// TODO: 启动实际的流式输出（将在 Phase 2 实现）
	// 暂时发送一条简单的响应然后完成
	return m, tea.Sequence(
		func() tea.Msg {
			return StreamDeltaMsg{
				MessageID: msg.MessageID,
				Delta:     "I understand you want me to help with that. ",
			}
		},
		func() tea.Msg {
			return StreamDeltaMsg{
				MessageID: msg.MessageID,
				Delta:     "AI integration will be implemented in Phase 2.\n",
			}
		},
		StreamComplete(msg.MessageID),
	)
}

// handleStreamDelta 处理流式增量
func (m Model) handleStreamDelta(msg StreamDeltaMsg) (tea.Model, tea.Cmd) {
	for i := range m.messages {
		if m.messages[i].ID == msg.MessageID {
			m.messages[i].AppendContent(msg.Delta)
			break
		}
	}

	// TODO: 实际的流式输出由 Agent 驱动，这里不需要主动生成
	return m, nil
}

// handleStreamComplete 处理流式完成
func (m Model) handleStreamComplete(msg StreamCompleteMsg) (tea.Model, tea.Cmd) {
	for i := range m.messages {
		if m.messages[i].ID == msg.MessageID {
			m.messages[i].Complete()
			break
		}
	}
	m.appState = StateIdle
	return m, nil
}

// handleStreamError 处理流式错误
func (m Model) handleStreamError(msg StreamErrorMsg) (tea.Model, tea.Cmd) {
	for i := range m.messages {
		if m.messages[i].ID == msg.MessageID {
			m.messages[i].SetError(msg.Error)
			break
		}
	}
	m.appState = StateError
	return m, nil
}

// handleReActStep 处理ReAct步骤
func (m Model) handleReActStep(msg ReActStepMsg) (tea.Model, tea.Cmd) {
	for i := range m.messages {
		if m.messages[i].ID == msg.MessageID {
			m.messages[i].AddReActStep(msg.StepType, msg.Content)
			break
		}
	}
	return m, nil
}

// handleToolCall 处理工具调用
func (m Model) handleToolCall(msg ToolCallMsg) (tea.Model, tea.Cmd) {
	for i := range m.messages {
		if m.messages[i].ID == msg.MessageID {
			m.messages[i].AddToolCall(msg.ToolName, msg.Input)
			break
		}
	}
	m.appState = StateExecutingTool
	return m, nil
}

// handleToolResult 处理工具结果
func (m Model) handleToolResult(msg ToolResultMsg) (tea.Model, tea.Cmd) {
	status := message.StatusCompleted
	if !msg.Success {
		status = message.StatusError
	}
	for i := range m.messages {
		if m.messages[i].ID == msg.MessageID {
			m.messages[i].UpdateToolCall(msg.ToolName, msg.Output, status, msg.Error)
			break
		}
	}
	m.appState = StateIdle
	return m, nil
}

// handlePermissionRequest 处理权限请求
func (m Model) handlePermissionRequest(msg PermissionRequestMsg) (tea.Model, tea.Cmd) {
	m.permission.Active = true
	m.permission.MessageID = msg.MessageID
	m.permission.ToolName = msg.ToolName
	m.permission.Operation = msg.Operation
	m.permission.Desc = msg.Desc
	m.permission.Cursor = 0
	m.appState = StateWaitingPermission
	return m, nil
}

// handlePermissionResponse 处理权限响应
func (m Model) handlePermissionResponse(msg PermissionResponseMsg) (tea.Model, tea.Cmd) {
	m.appState = StateIdle
	// TODO: 根据响应继续处理工具调用
	return m, nil
}

// handleInterrupt 处理中断
func (m Model) handleInterrupt() (tea.Model, tea.Cmd) {
	// 取消当前操作
	if m.currentMsgID != "" {
		for i := range m.messages {
			if m.messages[i].ID == m.currentMsgID {
				m.messages[i].Cancel()
				break
			}
		}
	}
	m.appState = StateIdle
	m.permission.Active = false
	return m, nil
}

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

		// 渲染ReAct循环
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

// renderReActStep 渲染ReAct步骤
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
