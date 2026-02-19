package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yourname/oxencode/internal/agent"
	"github.com/yourname/oxencode/internal/message"
	"github.com/yourname/oxencode/pkg/logger"
)

// buildContent 构建消息内容字符串
func (m *Model) buildContent() string {
	var b strings.Builder
	for _, msg := range m.messages {
		b.WriteString(m.renderMessage(msg))
		b.WriteString("\n\n")
	}
	return b.String()
}

// handleKeyMsg 处理键盘消息
func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

	case tea.KeyUp:
		// 如果输入框为空或按住 Alt，则向上滚动消息
		if m.input == "" || msg.Alt {
			if m.viewport.Height == 0 {
				return m, nil
			}

			// 设置最新内容
			content := m.buildContent()
			if content == "" {
				return m, nil
			}

			m.viewport.SetContent(content)

			if m.viewport.TotalLineCount() == 0 {
				return m, nil
			}

			m.viewport.ScrollUp(1)
			m.userScrolled = true
			m.atBottom = m.viewport.AtBottom()
			return m, nil
		}
		return m, nil

	case tea.KeyDown:
		// 如果输入框为空或按住 Alt，则向下滚动消息
		if m.input == "" || msg.Alt {
			// 设置最新内容
			content := m.buildContent()
			if content == "" {
				return m, nil
			}

			m.viewport.SetContent(content)
			m.viewport.ScrollDown(1)
			m.userScrolled = true
			m.atBottom = m.viewport.AtBottom()
			return m, nil
		}
		return m, nil

	case tea.KeyPgUp:
		// 如果输入框为空，则向上滚动半页
		if m.input == "" {
			content := m.buildContent()
			if content != "" {
				m.viewport.SetContent(content)
				m.viewport.HalfPageUp()
				m.userScrolled = true
				m.atBottom = m.viewport.AtBottom()
			}
			return m, nil
		}
		return m, nil

	case tea.KeyPgDown:
		// 如果输入框为空，则向下滚动半页
		if m.input == "" {
			content := m.buildContent()
			if content != "" {
				m.viewport.SetContent(content)
				m.viewport.HalfPageDown()
				m.userScrolled = true
				m.atBottom = m.viewport.AtBottom()
			}
			return m, nil
		}
		return m, nil

	default:
		// 处理字符输入（包括空格）
		if len(msg.Runes) > 0 {
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

	// 新消息到来，重置滚动状态，准备自动滚动
	m.userScrolled = false
	m.atBottom = true

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

	// 返回 ChatWithTools 开始命令，使用 ReAct 循环
	return m, func() tea.Msg {
		return ChatWithToolsStartMsg{
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

	// 继续等待原来的流式数据（用于 ChatStream，不是 ChatWithToolsWithProgress）
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

// handleChatWithToolsStart 处理 ChatWithTools 开始（使用 ReAct 循环）
func (m Model) handleChatWithToolsStart(msg ChatWithToolsStartMsg) (tea.Model, tea.Cmd) {
	// 创建 AI 消息用于跟踪整个 ReAct 循环
	aiMsg := message.NewStreamingMessage(message.RoleAssistant)
	aiMsg.ID = msg.MessageID
	m.messages = append(m.messages, aiMsg)
	m.currentMsgID = msg.MessageID
	m.appState = StateStreaming

	// 验证输入
	if m.agent == nil {
		m.appState = StateError
		aiMsg.SetError(fmt.Errorf("agent not available"))
		return m, nil
	}

	if msg.UserContent == "" {
		m.appState = StateError
		aiMsg.SetError(fmt.Errorf("user message is empty"))
		return m, nil
	}

	// 调用 Agent 获取 channel（Agent 负责创建）
	progressCh := m.agent.ChatWithToolsWithProgress(m.ctx, msg.UserContent)

	// 保存 channel 引用到 Model
	m.currentProgressCh = progressCh

	// 返回命令来监听 channel
	return m, waitForAgentProgress(msg.MessageID, progressCh)
}

// waitForAgentProgress 等待进度更新（单一 channel 模式）
func waitForAgentProgress(messageID string, progressCh <-chan agent.ProgressUpdate) tea.Cmd {
	log := logger.New("ui.progress")
	return func() tea.Msg {
		update, ok := <-progressCh
		if !ok {
			// Channel 关闭，正常结束
			log.Info("channel closed", "messageID", messageID)
			return nil
		}

		log.Info("progress update received",
			"messageID", messageID,
			"type", update.Type,
			"contentLength", len(update.Content),
			"toolName", update.ToolName)

		switch update.Type {
		case "thought":
			return ReActStepMsg{
				MessageID: messageID,
				StepType:  "thought",
				Content:   update.Content,
			}
		case "action":
			return ReActStepMsg{
				MessageID: messageID,
				StepType:  "action",
				Content:   update.Content,
				ToolName:  update.ToolName,
			}
		case "observation":
			return ReActStepMsg{
				MessageID: messageID,
				StepType:  "observation",
				Content:   update.Content,
				ToolName:  update.ToolName,
			}
		case "content":
			// 在 ReAct 循环中，content 应该作为 thinking 步骤显示
			// 只有 done 消息中的内容才是最终回答
			return ReActStepMsg{
				MessageID: messageID,
				StepType:  "thought",
				Content:   update.Content,
			}
		case "error":
			return ChatWithToolsCompleteMsg{
				MessageID: messageID,
				Error:     fmt.Errorf("agent error: %s", update.Content),
			}
		case "done":
			log.Info("done received", "messageID", messageID, "responseLength", len(update.Content))
			return ChatWithToolsCompleteMsg{
				MessageID: messageID,
				Response:  update.Content,
			}
		}
		return nil
	}
}

// handleChatWithToolsComplete 处理 ChatWithTools 完成
func (m Model) handleChatWithToolsComplete(msg ChatWithToolsCompleteMsg) (tea.Model, tea.Cmd) {
	log := logger.New("ui.complete")
	log.Info("ChatWithToolsComplete", "messageID", msg.MessageID, "responseLength", len(msg.Response))
	// 清理 channel 引用
	m.currentProgressCh = nil

	// 查找并更新消息
	for i := range m.messages {
		if m.messages[i].ID == msg.MessageID {
			log.Info("Found message to complete", "reactLoopLength", len(m.messages[i].ReActLoop))
			if msg.Error != nil {
				m.messages[i].SetError(msg.Error)
				m.appState = StateError
			} else {
				m.messages[i].AppendContent(msg.Response)
				m.messages[i].Complete()
				m.appState = StateIdle
			}
			log.Info("Message completed", "reactLoopLength", len(m.messages[i].ReActLoop), "contentLength", len(m.messages[i].Content))
			break
		}
	}
	return m, nil
}

// handleReActStep 处理 ReAct 步骤
func (m Model) handleReActStep(msg ReActStepMsg) (tea.Model, tea.Cmd) {
	log := logger.New("ui.reactStep")
	log.Info("handleReActStep called",
		"messageID", msg.MessageID,
		"stepType", msg.StepType,
		"contentLength", len(msg.Content),
		"toolName", msg.ToolName,
		"messagesCount", len(m.messages))

	for i := range m.messages {
		if m.messages[i].ID == msg.MessageID {
			log.Info("Found matching message", "index", i, "currentReActLoopLength", len(m.messages[i].ReActLoop))
			switch msg.StepType {
			case "action":
				// msg.Content 已经是格式化的工具调用字符串，如 glob("**/*.go")
				// 创建工具调用，将格式化字符串保存为 display 字段
				input := map[string]any{"display": msg.Content}
				m.messages[i].AddToolCall(msg.ToolName, input)
				log.Info("Added tool call", "toolName", msg.ToolName, "display", msg.Content, "newReActLoopLength", len(m.messages[i].ReActLoop))
			case "observation":
				// observation 类型需要更新工具调用结果
				status := message.StatusCompleted
				// 检查是否是错误观察（包含 "failed" 或 "Error" 字样）
				isError := strings.Contains(msg.Content, "failed") || strings.Contains(msg.Content, "Error") || strings.Contains(msg.Content, "is a directory")
				if isError {
					status = message.StatusError
					log.Info("Detected error in observation", "content", msg.Content, "toolName", msg.ToolName)
				}
				m.messages[i].UpdateToolCall(msg.ToolName, msg.Content, status, "")
				log.Info("Updated tool call", "toolName", msg.ToolName, "status", status, "reactLoopLength", len(m.messages[i].ReActLoop))
			default:
				m.messages[i].AddReActStep(msg.StepType, msg.Content)
				log.Info("Added ReAct step", "stepType", msg.StepType, "newReActLoopLength", len(m.messages[i].ReActLoop))
			}
			break
		}
	}

	// 继续监听进度（如果 channel 仍可用）
	if m.currentProgressCh != nil {
		return m, waitForAgentProgress(msg.MessageID, m.currentProgressCh)
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
