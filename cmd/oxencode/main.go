package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yourname/oxencode/internal/ui"
)

func main() {
	// 创建 Bubble Tea 程序
	p := tea.NewProgram(
		ui.NewModel(),
		tea.WithAltScreen(),       // 使用备用屏幕（全屏模式）
		tea.WithMouseCellMotion(), // 启用鼠标支持
	)

	// 运行程序
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
