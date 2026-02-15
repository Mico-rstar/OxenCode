package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Styles 定义所有UI样式
type Styles struct {
	// 通用样式
	Normal       lipgloss.Style
	Bold         lipgloss.Style
	Dim          lipgloss.Style
	Italic       lipgloss.Style

	// 颜色样式
	Primary      lipgloss.Style // 主色调
	Secondary    lipgloss.Style // 次要色调
	Muted        lipgloss.Style // 弱化文本
	Error        lipgloss.Style // 错误文本
	Warning      lipgloss.Style // 警告文本
	Success      lipgloss.Style // 成功文本

	// 布局样式
	App          lipgloss.Style // 整个应用
	Header       lipgloss.Style // 顶部状态栏
	Footer       lipgloss.Style // 底部输入区域
	MessageArea  lipgloss.Style // 消息显示区域

	// 消息样式
	UserMsg      lipgloss.Style // 用户消息
	AssistantMsg lipgloss.Style // AI消息
	SystemMsg    lipgloss.Style // 系统消息
	ToolMsg      lipgloss.Style // 工具消息
	ThoughtMsg   lipgloss.Style // 思考消息

	// ReAct步骤样式
	ReActStep    lipgloss.Style
	ReActBranch  lipgloss.Style
	ReActLeaf    lipgloss.Style

	// 输入框样式
	InputBox     lipgloss.Style
	InputPrompt  lipgloss.Style
	InputHelp    lipgloss.Style

	// 弹窗样式
	Modal        lipgloss.Style
	ModalTitle   lipgloss.Style
	ModalBorder  lipgloss.Style
	ModalButton  lipgloss.Style
	ModalButtonFocus lipgloss.Style

	// 状态指示器
	StatusOnline lipgloss.Style
	StatusOffline lipgloss.Style
	StatusBusy   lipgloss.Style
	StatusError  lipgloss.Style

	// 图标
	IconUser     lipgloss.Style
	IconAssistant lipgloss.Style
	IconTool     lipgloss.Style
	IconThought  lipgloss.Style
	IconWarning  lipgloss.Style
	IconSuccess  lipgloss.Style
	IconError    lipgloss.Style
	IconLoading  lipgloss.Style
}

// DefaultStyles 返回默认样式配置
func DefaultStyles() *Styles {
	s := &Styles{}

	// 颜色定义
	primaryColor := lipgloss.Color("86")    // 青色
	secondaryColor := lipgloss.Color("98")  // 紫色
	mutedColor := lipgloss.Color("245")     // 灰色
	errorColor := lipgloss.Color("196")     // 红色
	warningColor := lipgloss.Color("226")   // 黄色
	successColor := lipgloss.Color("142")   // 绿色

	// 通用样式
	s.Normal = lipgloss.NewStyle()
	s.Bold = lipgloss.NewStyle().Bold(true)
	s.Dim = lipgloss.NewStyle().Faint(true)
	s.Italic = lipgloss.NewStyle().Italic(true)

	// 颜色样式
	s.Primary = lipgloss.NewStyle().Foreground(primaryColor)
	s.Secondary = lipgloss.NewStyle().Foreground(secondaryColor)
	s.Muted = lipgloss.NewStyle().Foreground(mutedColor)
	s.Error = lipgloss.NewStyle().Foreground(errorColor)
	s.Warning = lipgloss.NewStyle().Foreground(warningColor)
	s.Success = lipgloss.NewStyle().Foreground(successColor)

	// 布局样式
	s.App = lipgloss.NewStyle().
		Padding(0, 1)

	s.Header = lipgloss.NewStyle().
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("62")).
		Bold(true).
		Padding(0, 1)

	s.Footer = lipgloss.NewStyle().
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("240")).
		Padding(0, 1)

	s.MessageArea = lipgloss.NewStyle().
		Padding(1, 0).
		MarginBottom(1)

	// 消息样式
	s.UserMsg = lipgloss.NewStyle().
		Foreground(lipgloss.Color("51")).
		Bold(true).
		Padding(0, 1).
		MarginBottom(1)

	s.AssistantMsg = lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true).
		Padding(0, 1).
		MarginBottom(1)

	s.SystemMsg = lipgloss.NewStyle().
		Foreground(mutedColor).
		Italic(true).
		Padding(0, 1).
		MarginBottom(1)

	s.ToolMsg = lipgloss.NewStyle().
		Foreground(warningColor).
		Padding(0, 2)

	s.ThoughtMsg = lipgloss.NewStyle().
		Foreground(lipgloss.Color("117")).
		Italic(true).
		Padding(0, 3)

	// ReAct步骤样式
	s.ReActStep = lipgloss.NewStyle().
		Padding(0, 1)

	s.ReActBranch = lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Padding(0, 1)

	s.ReActLeaf = lipgloss.NewStyle().
		Padding(0, 2)

	// 输入框样式
	s.InputBox = lipgloss.NewStyle().
		Padding(1, 2).
		Background(lipgloss.Color("235")).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238"))

	s.InputPrompt = lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true)

	s.InputHelp = lipgloss.NewStyle().
		Foreground(mutedColor).
		Faint(true)

	// 弹窗样式
	s.Modal = lipgloss.NewStyle().
		Padding(2, 4).
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("62")).
		Background(lipgloss.Color("236"))

	s.ModalTitle = lipgloss.NewStyle().
		Foreground(warningColor).
		Bold(true).
		MarginBottom(1)

	s.ModalBorder = lipgloss.NewStyle().
		Foreground(lipgloss.Color("62"))

	s.ModalButton = lipgloss.NewStyle().
		Padding(0, 2).
		Foreground(lipgloss.Color("245")).
		Background(lipgloss.Color("236"))

	s.ModalButtonFocus = lipgloss.NewStyle().
		Padding(0, 2).
		Foreground(lipgloss.Color("230")).
		Background(primaryColor).
		Bold(true)

	// 状态指示器
	s.StatusOnline = lipgloss.NewStyle().Foreground(successColor)
	s.StatusOffline = lipgloss.NewStyle().Foreground(mutedColor)
	s.StatusBusy = lipgloss.NewStyle().Foreground(warningColor)
	s.StatusError = lipgloss.NewStyle().Foreground(errorColor)

	// 图标样式
	s.IconUser = lipgloss.NewStyle().Foreground(lipgloss.Color("51"))
	s.IconAssistant = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	s.IconTool = lipgloss.NewStyle().Foreground(warningColor)
	s.IconThought = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	s.IconWarning = lipgloss.NewStyle().Foreground(warningColor)
	s.IconSuccess = lipgloss.NewStyle().Foreground(successColor)
	s.IconError = lipgloss.NewStyle().Foreground(errorColor)
	s.IconLoading = lipgloss.NewStyle().Foreground(lipgloss.Color("228"))

	return s
}

// 图标常量
const (
	IconUser      = "🧒"
	IconAssistant = "🤖"
	IconSystem    = "⚙️"
	IconTool      = "🔧"
	IconThought   = "💭"
	IconWarning   = "⚠️"
	IconSuccess   = "✅"
	IconError     = "❌"
	IconLoading   = "⠋"

	StatusOnline  = "●"
	StatusOffline = "○"
	StatusBusy    = "◐"
	StatusError   = "✖"
)
