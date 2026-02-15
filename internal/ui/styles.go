package ui

import (
	"github.com/charmbracelet/lipgloss"
	"os"
	"strconv"
)

var (
	// TitleStyle 标题样式
	TitleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true).
		MarginTop(1).
		MarginBottom(1)

	// SubtitleStyle 副标题样式
	SubtitleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		MarginBottom(1)

	// InputInnerStyle 输入框内部样式
	InputInnerStyle = lipgloss.NewStyle()

	// MessageStyle 消息区域样式
	MessageStyle = lipgloss.NewStyle().
		MarginBottom(1)

	// UserMsgStyle 用户消息样式
	UserMsgStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("42")).
		Bold(true)

	// AIMsgStyle AI 消息样式
	AIMsgStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("226"))

	// SystemMsgStyle 系统消息样式
	SystemMsgStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("243")).
		Italic(true)

	// StatusStyle 状态栏样式
	StatusStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Background(lipgloss.Color("235")).
		Padding(0, 1)

	// HelpStyle 帮助提示样式
	HelpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)

	// useSimpleBorders 是否使用简单边框
	useSimpleBorders bool
)

// InputContainerStyle 输入框容器样式
var InputContainerStyle = lipgloss.NewStyle().
	Border(getBorderStyle()).
	BorderForeground(lipgloss.Color("62")).
	Padding(0, 1)

func init() {
	useSimpleBorders = shouldUseSimpleBorders()
	InputContainerStyle = lipgloss.NewStyle().
		Border(getBorderStyle()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1)
}

// getBorderStyle 根据环境返回边框样式
func getBorderStyle() lipgloss.Border {
	if useSimpleBorders {
		return lipgloss.NormalBorder()
	}
	return lipgloss.RoundedBorder()
}

// shouldUseSimpleBorders 检测是否应该使用简单边框
func shouldUseSimpleBorders() bool {
	// 检查环境变量
	if os.Getenv("OXENCODE_SIMPLE_BORDER") == "1" {
		return true
	}

	// 检查 TERM 环境变量
	term := os.Getenv("TERM")
	if term == "" || term == "dumb" {
		return true
	}

	// 检查是否在特定的可能不支持 Unicode 的终端中
	// 可以根据需要添加更多检测
	return false
}

// 欢迎信息
const (
	WelcomeTitle   = "╔════════════════════════════════════════════╗"
	WelcomeTitle2  = "║      OxenCode AI Assistant                  ║"
	WelcomeTitle3  = "╚════════════════════════════════════════════╝"
	WelcomeMessage = "Welcome to OxenCode! Your AI programming assistant."
	WelcomeHint    = "Type a message and press Enter to start. Press Ctrl+C to quit."
)

// 简单边框的欢迎信息
const (
	WelcomeTitleSimple   = "+==========================================+"
	WelcomeTitle2Simple  = "|      OxenCode AI Assistant          |"
	WelcomeTitle3Simple  = "+==========================================+"
)

// GetWelcomeStyle 获取欢迎信息样式
func GetWelcomeStyle() string {
	if useSimpleBorders {
		title := TitleStyle.Render(WelcomeTitleSimple + "\n" + WelcomeTitle2Simple + "\n" + WelcomeTitle3Simple)
		message := SubtitleStyle.Render(WelcomeMessage)
		hint := HelpStyle.Render(WelcomeHint)
		return title + "\n" + message + "\n" + hint + "\n"
	}

	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true).
		MarginBottom(1).
		Render(WelcomeTitle + "\n" + WelcomeTitle2 + "\n" + WelcomeTitle3)

	message := SubtitleStyle.Render(WelcomeMessage)
	hint := HelpStyle.Render(WelcomeHint)

	return title + "\n" + message + "\n" + hint + "\n"
}

// SetSimpleBorder 设置是否使用简单边框（用于配置）
func SetSimpleBorder(simple bool) {
	useSimpleBorders = simple
	// 更新样式
	InputContainerStyle = lipgloss.NewStyle().
		Border(getBorderStyle()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1)
}

// GetTerminalWidth 获取终端宽度
func GetTerminalWidth() int {
	if width := os.Getenv("COLUMNS"); width != "" {
		if w, err := strconv.Atoi(width); err == nil {
			return w
		}
	}
	return 80 // 默认宽度
}
