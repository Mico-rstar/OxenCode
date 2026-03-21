package commands

import (
	"context"
	"fmt"

	"github.com/yourname/oxencode/internal/cli/slashcmd"
)

// CommitMemoryCommand 提交会话到记忆服务命令
type CommitMemoryCommand struct{}

// Name 返回命令名称
func (c *CommitMemoryCommand) Name() string {
	return "commit-memory"
}

// Description 返回命令描述
func (c *CommitMemoryCommand) Description() string {
	return "Commit current session to memory service and create a new session"
}

// Execute 执行命令
func (c *CommitMemoryCommand) Execute(ctx context.Context, cmdCtx slashcmd.CommandContext, args string) (string, error) {
	// 检查记忆服务是否启用
	if !cmdCtx.IsMemoryEnabled() {
		return "错误：记忆服务未启用。\n请在配置中设置 memory_enabled = true", nil
	}

	// 保存旧会话 ID 用于显示
	oldSessionID := cmdCtx.GetSessionID()

	// 提交会话到记忆服务
	taskID, err := cmdCtx.CommitSessionToMemory(ctx)
	if err != nil {
		return "", fmt.Errorf("提交失败：%w", err)
	}

	// 提交成功，尝试创建新会话
	if err := cmdCtx.NewSession(); err != nil {
		// 创建新会话失败，保留原会话
		return fmt.Sprintf("会话已提交到记忆服务，但创建新会话失败。\n"+
			"Session ID: %s\n"+
			"Task ID: %s\n"+
			"创建新会话错误：%v\n"+
			"您仍可继续使用当前会话。", oldSessionID, taskID, err), nil
	}

	// 成功：提交并创建新会话
	newSessionID := cmdCtx.GetSessionID()
	return fmt.Sprintf("会话已提交到记忆服务，并创建了新会话。\n"+
		"已提交会话 ID: %s\n"+
		"Task ID: %s\n"+
		"新会话 ID: %s", oldSessionID, taskID, newSessionID), nil
}

// NewCommitMemoryCommand 创建命令实例
func NewCommitMemoryCommand() *CommitMemoryCommand {
	return &CommitMemoryCommand{}
}

// Ensure CommitMemoryCommand implements SlashCommand
var _ slashcmd.SlashCommand = (*CommitMemoryCommand)(nil)
