package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yourname/oxencode/internal/cli"
	"github.com/yourname/oxencode/internal/ui"
	"github.com/yourname/oxencode/pkg/config"
	"github.com/yourname/oxencode/pkg/logger"
)

func main() {
	// 解析命令行参数
	simpleMode := flag.Bool("simple", false, "Use simple CLI mode instead of TUI")
	simpleShort := flag.Bool("s", false, "Use simple CLI mode (shorthand)")
	flag.Parse()

	useSimple := *simpleMode || *simpleShort

	// 初始化日志系统
	if err := logger.InitFromEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting OxenCode",
		"version", "1.0.0",
		"mode", map[bool]string{true: "simple", false: "tui"}[useSimple],
	)

	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if useSimple {
		// 简单 CLI 模式
		runSimpleCLI(cfg)
	} else {
		// TUI 模式
		runTUI()
	}

	logger.Info("OxenCode exited normally")
}

// runSimpleCLI 运行简单 CLI 模式
func runSimpleCLI(cfg *config.Config) {
	c, err := cli.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating CLI: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()

	// 运行 REPL
	if err := c.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// runTUI 运行 TUI 模式
func runTUI() {
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
}