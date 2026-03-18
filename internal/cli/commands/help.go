package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/yourname/oxencode/internal/cli/slashcmd"
)

// HelpCommand 帮助命令
type HelpCommand struct {
	registry slashcmd.CommandRegistryReader
}

// Name 返回命令名称
func (c *HelpCommand) Name() string {
	return "help"
}

// Description 返回命令描述
func (c *HelpCommand) Description() string {
	return "Show available slash commands"
}

// Execute 执行帮助命令
func (c *HelpCommand) Execute(ctx context.Context, cmdCtx slashcmd.CommandContext, args string) (string, error) {
	var sb strings.Builder
	sb.WriteString("Available commands:\n")

	for _, name := range c.registry.Names() {
		cmd := c.registry.Get(name)
		sb.WriteString(fmt.Sprintf("  /%-10s - %s\n", name, cmd.Description()))
	}

	sb.WriteString("\nTip: Commands must start with '/' (e.g., /clear)")
	return sb.String(), nil
}

// NewHelpCommand 创建帮助命令实例
func NewHelpCommand(registry slashcmd.CommandRegistryReader) *HelpCommand {
	return &HelpCommand{registry: registry}
}

// Ensure HelpCommand implements SlashCommand
var _ slashcmd.SlashCommand = (*HelpCommand)(nil)