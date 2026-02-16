package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yourname/oxencode/internal/message"
)

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
	// 创建并添加用户消息
	userMsg := message.NewMessage(message.RoleUser, msg.Content)
	m.messages = append(m.messages, userMsg)

	// 清空输入框
	m.input = ""
	m.cursor = 0
	m.appState = StateThinking

	// 如果 Agent 不可用，返回错误
	if m.agent == nil {
		m.appState = StateError
		return m, nil
	}

	// 生成 AI 消息 ID
	msgID := message.GenerateID()
	m.currentMsgID = msgID

	// 返回流式开始命令，携带用户消息内容
	return m, func() tea.Msg {
		return StreamStartMsg{
			MessageID:   msgID,
			UserContent: msg.Content,
		}
	}
}

// handleStreamStart 处理流式开始
func (m Model) handleStreamStart(msg StreamStartMsg) (tea.Model, tea.Cmd) {
	// 创建流式 AI 消息
	aiMsg := message.NewStreamingMessage(message.RoleAssistant)
	aiMsg.ID = msg.MessageID
	m.messages = append(m.messages, aiMsg)
	m.currentMsgID = msg.MessageID
	m.appState = StateStreaming

	// 验证输入
	if m.agent == nil {
		m.appState = StateError
		return m, StreamError(msg.MessageID, fmt.Errorf("agent not available"))
	}

	if msg.UserContent == "" {
		m.appState = StateError
		return m, StreamError(msg.MessageID, fmt.Errorf("user message is empty"))
	}

	// 启动 Agent 流式对话
	streamCh, errCh := m.agent.ChatStream(m.ctx, msg.UserContent)

	// 保存 channels 到 Model，以便后续继续监听
	m.streamCh = streamCh
	m.errCh = errCh

	// 返回一个命令来处理流式响应
	return m, m.waitForStreamData(msg.MessageID)
}

// handleStreamDelta 处理流式增量
func (m Model) handleStreamDelta(msg StreamDeltaMsg) (tea.Model, tea.Cmd) {
	for i := range m.messages {
		if m.messages[i].ID == msg.MessageID {
			m.messages[i].AppendContent(msg.Delta)
			break
		}
	}

	// 继续等待下一个流式数据
	return m, m.waitForStreamData(msg.MessageID)
}

// waitForStreamData 等待流式数据的辅助函数
func (m Model) waitForStreamData(messageID string) tea.Cmd {
	return func() tea.Msg {
		select {
		case delta, ok := <-m.streamCh:
			if !ok {
				// 流结束
				return StreamCompleteMsg{MessageID: messageID}
			}
			return StreamDeltaMsg{MessageID: messageID, Delta: delta}
		case err := <-m.errCh:
			if err != nil {
				return StreamErrorMsg{MessageID: messageID, Error: err}
			}
			return StreamCompleteMsg{MessageID: messageID}
		case <-m.ctx.Done():
			// 上下文被取消
			return StreamErrorMsg{MessageID: messageID, Error: fmt.Errorf("cancelled")}
		}
	}
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

// handleReActStep 处理 ReAct 步骤
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
func (m Model) handlePermissionResponse(_ PermissionResponseMsg) (tea.Model, tea.Cmd) {
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
