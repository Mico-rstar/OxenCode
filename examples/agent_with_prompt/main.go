package main

import (
	"context"
	"fmt"
	"os"

	"github.com/yourname/oxencode/internal/agent"
	"github.com/yourname/oxencode/pkg/config"
)

func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("=== OxenCode Agent with Dynamic Prompt Loading ===\n\n")
	fmt.Printf("Configuration:\n")
	fmt.Printf("  Provider: %s\n", cfg.Provider)
	fmt.Printf("  Model: %s\n", cfg.Model)
	fmt.Printf("  Prompt Dir: %s\n", cfg.PromptDir)
	fmt.Printf("  Work Dir: %s\n\n", cfg.WorkDir)

	// 创建 Agent（会自动从配置的 prompt_dir 加载系统提示词）
	ag, err := agent.NewAgent(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create agent: %v\n", err)
		os.Exit(1)
	}

	// 显示当前系统提示词信息
	history := ag.GetHistory()
	if len(history) > 0 {
		systemMsg := history[0]
		fmt.Printf("=== System Prompt Loaded ===\n")
		fmt.Printf("Source: %s\n", cfg.PromptDir)
		fmt.Printf("Length: %d characters\n", len(systemMsg.Content))
		fmt.Printf("\nFirst 200 chars:\n%s\n\n", truncate(systemMsg.Content, 200))
	}

	// 演示重新加载系统提示词
	fmt.Printf("=== Demonstrating Dynamic Prompt Reload ===\n")
	fmt.Printf("Reloading system prompt...\n")
	if err := ag.ReloadSystemPrompt(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to reload prompt: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("System prompt reloaded successfully!\n\n")

	// 简单对话测试
	fmt.Printf("=== Testing Agent Response ===\n")
	fmt.Printf("User: What tools do you have available?\n\n")

	ctx := context.Background()

	// 使用 ChatWithToolsWithProgress
	progressCh := ag.ChatWithToolsWithProgress(ctx, "What tools do you have available? Please list them.")

	var response string
	for update := range progressCh {
		switch update.Type {
		case "action":
			fmt.Printf("[Action] %s\n", update.Content)
		case "observation":
			fmt.Printf("[Observation] %s\n", update.Content)
		case "error":
			fmt.Fprintf(os.Stderr, "Error: %s\n", update.Content)
			os.Exit(1)
		case "done":
			response = update.Content
		}
	}

	fmt.Printf("Agent: %s\n", response)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
