package commands

import (
	"context"

	"github.com/yourname/oxencode/internal/cli/slashcmd"
)

// ClearCommand 清空会话命令
type ClearCommand struct{}

// Name 返回命令名称
func (c *ClearCommand) Name() string {
	return "clear"
}

// Description 返回命令描述
func (c *ClearCommand) Description() string {
	return "Clear the current conversation history"
}

// Execute 执行清空命令
func (c *ClearCommand) Execute(ctx context.Context, cmdCtx slashcmd.CommandContext, args string) (string, error) {
	cmdCtx.ClearSession()
	return "Conversation history cleared.", nil
}

// NewClearCommand 创建清空命令实例
func NewClearCommand() *ClearCommand {
	return &ClearCommand{}
}

// Ensure ClearCommand implements SlashCommand
var _ slashcmd.SlashCommand = (*ClearCommand)(nil)