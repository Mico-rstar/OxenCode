// Package cli 提供简单的命令行交互界面
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/peterh/liner"
	"github.com/yourname/oxencode/internal/agent"
	"github.com/yourname/oxencode/pkg/config"
)

// CLI 简单命令行界面
type CLI struct {
	agent  *agent.Agent
	liner  *liner.State
	writer io.Writer
}

// New 创建新的 CLI 实例
func New(cfg *config.Config) (*CLI, error) {
	ag, err := agent.NewAgent(cfg)
	ag.EnableSnapshot("snapshot")
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	return &CLI{
		agent:  ag,
		liner:  liner.NewLiner(),
		writer: os.Stdout,
	}, nil
}

// Run 启动 REPL 循环
func (c *CLI) Run(ctx context.Context) error {
	defer c.liner.Close()

	fmt.Fprintln(c.writer, "OxenCode Simple CLI")
	fmt.Fprintln(c.writer, "Type 'exit' or 'quit' to exit")
	fmt.Fprintln(c.writer, "")

	// 设置 liner 选项
	c.liner.SetCtrlCAborts(true) // Ctrl+C 返回 io.EOF

	for {
		// 使用 goroutine 包装 liner.Prompt 以支持 context 取消
		inputCh := make(chan string, 1)
		errCh := make(chan error, 1)

		go func() {
			input, err := c.liner.Prompt("> ")
			if err != nil {
				errCh <- err
			} else {
				inputCh <- input
			}
		}()

		select {
		case <-ctx.Done():
			fmt.Fprintln(c.writer, "\nGoodbye!")
			return nil
		case err := <-errCh:
			if err == io.EOF {
				fmt.Fprintln(c.writer, "\nGoodbye!")
				return nil
			}
			if err == liner.ErrPromptAborted {
				// Ctrl+C 被按下，退出程序
				fmt.Fprintln(c.writer, "^C")
				return nil
			}
			return fmt.Errorf("failed to read input: %w", err)
		case input := <-inputCh:
			// 处理输入
			input = strings.TrimSpace(input)
			if input == "" {
				continue
			}

			// 添加到历史记录
			c.liner.AppendHistory(input)

			// 处理退出命令
			if input == "exit" || input == "quit" {
				fmt.Fprintln(c.writer, "Goodbye!")
				return nil
			}

			// 处理消息
			c.processMessage(ctx, input)
		}
	}
}

// processMessage 处理用户消息
// 对于流式内容（thought, content），采用累积策略：
// - thought: 累积后一次性输出（在 action 或 done 时触发）
// - content: 实时打印（打字机效果）
// - action/observation: 立即输出
func (c *CLI) processMessage(ctx context.Context, input string) {
	ch := c.agent.ChatWithToolsWithProgress(ctx, input)

	// 累积器
	var thoughtBuf strings.Builder
	var contentBuf strings.Builder

	// 状态跟踪
	inThought := false
	inContent := false

	// 辅助函数：flush 思考内容
	flushThought := func() {
		if thoughtBuf.Len() > 0 {
			fmt.Fprintf(c.writer, "\n[思考] %s\n", thoughtBuf.String())
			thoughtBuf.Reset()
		}
		inThought = false
	}

	// 辅助函数：flush 内容
	flushContent := func() {
		if contentBuf.Len() > 0 {
			fmt.Fprintln(c.writer, "") // 换行
			contentBuf.Reset()
		}
		inContent = false
	}

	for update := range ch {
		switch update.Type {
		case "thought":
			// 如果之前在 content 模式，先结束
			if inContent {
				flushContent()
			}
			// 累积思考内容
			thoughtBuf.WriteString(update.Content)
			inThought = true

		case "action":
			// action 前先 flush 思考
			if inThought {
				flushThought()
			}
			if inContent {
				flushContent()
			}
			c.printAction(update.ToolName, update.Content)

		case "observation":
			// observation 前先 flush 思考
			if inThought {
				flushThought()
			}
			c.printObservation(update.ToolName, update.Content)

		case "content":
			// 如果之前在 thought 模式，先结束
			if inThought {
				flushThought()
			}
			// 首次进入 content 模式，打印标签
			if !inContent {
				fmt.Fprintln(c.writer, "\n[回答]")
				inContent = true
			}
			// 实时打印内容（打字机效果）
			fmt.Fprint(c.writer, update.Content)
			contentBuf.WriteString(update.Content)

		case "error":
			if inThought {
				flushThought()
			}
			if inContent {
				flushContent()
			}
			c.printError(update.Content)

		case "done":
			if inThought {
				flushThought()
			}
			if inContent {
				flushContent()
			}
			// done 仅表示完成，内容已通过流式 content 打印
			fmt.Fprintln(c.writer, "")
		}
	}

	// 确保 flush 任何残留内容
	if inThought {
		flushThought()
	}
	if inContent {
		flushContent()
	}
}

// printAction 打印工具调用
func (c *CLI) printAction(toolName, content string) {
	fmt.Fprintf(c.writer, "\n[工具] %s", toolName)
	if content != "" && len(content) < 100 {
		fmt.Fprintf(c.writer, "(%s)", content)
	}
	fmt.Fprintln(c.writer)
}

// printObservation 打印工具结果
func (c *CLI) printObservation(toolName, content string) {
	// 截断长输出
	display := content
	if len(display) > 200 {
		display = display[:200] + "..."
	}
	// 替换换行符以便单行显示
	display = strings.ReplaceAll(display, "\n", " ")
	fmt.Fprintf(c.writer, "  -> %s\n", display)
}

// printError 打印错误信息
func (c *CLI) printError(content string) {
	fmt.Fprintf(c.writer, "\n[错误] %s\n", content)
}

// Close 关闭 CLI，释放资源
func (c *CLI) Close() {
	if c.liner != nil {
		c.liner.Close()
	}
	if c.agent != nil {
		c.agent.Close()
	}
}