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
	return "Commit current session to memory service"
}

// Execute 执行命令
func (c *CommitMemoryCommand) Execute(ctx context.Context, cmdCtx slashcmd.CommandContext, args string) (string, error) {
	// 检查记忆服务是否启用
	if !cmdCtx.IsMemoryEnabled() {
		return "错误: 记忆服务未启用。\n请在配置中设置 memory_enabled = true", nil
	}

	// 提交会话到记忆服务
	taskID, err := cmdCtx.CommitSessionToMemory(ctx)
	if err != nil {
		return "", fmt.Errorf("提交失败: %w", err)
	}

	return fmt.Sprintf("会话已提交到记忆服务。\nSession ID: %s\nTask ID: %s", cmdCtx.GetSessionID(), taskID), nil
}

// NewCommitMemoryCommand 创建命令实例
func NewCommitMemoryCommand() *CommitMemoryCommand {
	return &CommitMemoryCommand{}
}

// Ensure CommitMemoryCommand implements SlashCommand
var _ slashcmd.SlashCommand = (*CommitMemoryCommand)(nil)