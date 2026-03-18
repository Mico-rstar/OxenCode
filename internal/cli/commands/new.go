package commands

import (
	"context"
	"fmt"

	"github.com/yourname/oxencode/internal/cli/slashcmd"
)

// NewCommand 创建新会话命令
type NewCommand struct{}

// Name 返回命令名称
func (c *NewCommand) Name() string {
	return "new"
}

// Description 返回命令描述
func (c *NewCommand) Description() string {
	return "Create a new conversation session"
}

// Execute 执行新建会话命令
func (c *NewCommand) Execute(ctx context.Context, cmdCtx slashcmd.CommandContext, args string) (string, error) {
	oldID := cmdCtx.GetSessionID()
	if err := cmdCtx.NewSession(); err != nil {
		return "", fmt.Errorf("failed to create new session: %w", err)
	}
	newID := cmdCtx.GetSessionID()
	return fmt.Sprintf("New session created. Previous: %s, Current: %s", oldID, newID), nil
}

// NewNewCommand 创建新建会话命令实例
func NewNewCommand() *NewCommand {
	return &NewCommand{}
}

// Ensure NewCommand implements SlashCommand
var _ slashcmd.SlashCommand = (*NewCommand)(nil)