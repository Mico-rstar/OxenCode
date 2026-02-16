package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yourname/oxencode/internal/agent"
	"github.com/yourname/oxencode/internal/config"
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

	// Agent 相关
	agent        *agent.Agent
	currentMsgID string // 当前正在处理的消息ID
	ctx          context.Context
	cancelFunc   context.CancelFunc

	// 流式响应 channels
	streamCh     <-chan string
	errCh        <-chan error

	// 状态栏
	statusTime  string
	statusConn  string
	statusModel string
}

// NewModel 创建新模型
func NewModel() Model {
	styles := DefaultStyles()

	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		// 配置加载失败，显示错误消息
		return Model{
			messages: []message.Message{
				message.NewMessage(
					message.RoleSystem,
					"Error loading config: "+err.Error()+"\n\nPlease set API key environment variable.",
				),
			},
			appState: StateError,
			err:      err,
			styles:   styles,
			width:    80,
			height:   24,
		}
	}

	// 创建 Agent
	ag, err := agent.NewAgent(cfg)
	if err != nil {
		// Agent 创建失败（可能是没有 API key）
		return Model{
			messages: []message.Message{
				message.NewMessage(
					message.RoleSystem,
					"Error: "+err.Error()+"\n\nPlease set API key environment variable and restart.",
				),
			},
			appState: StateError,
			err:      err,
			styles:   styles,
			width:    80,
			height:   24,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

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
		agent:       ag,
		ctx:         ctx,
		cancelFunc:  cancel,
		statusTime:  time.Now().Format("15:04:05"),
		statusConn:  StatusOnline,
		statusModel: cfg.Model,
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

	return m, nil
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
