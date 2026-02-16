package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yourname/oxencode/internal/ui"
	"github.com/yourname/oxencode/pkg/logger"
)

func main() {
	// 初始化日志系统
	if err := logger.InitFromEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting OxenCode",
		"version", "1.0.0",
	)

	// 创建 Bubble Tea 程序
	p := tea.NewProgram(
		ui.NewModel(),
		tea.WithAltScreen(),       // 使用备用屏幕（全屏模式）
		tea.WithMouseCellMotion(), // 启用鼠标支持
	)

	// 运行程序
	if _, err := p.Run(); err != nil {
		logger.Error("Program error", "error", err)
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	logger.Info("OxenCode exited normally")
}
