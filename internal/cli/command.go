package cli

import (
	"fmt"
	"sort"
	"sync"

	"github.com/yourname/oxencode/internal/cli/slashcmd"
)

// CommandRegistry 斜杠命令注册表
// 管理所有可用的斜杠命令
type CommandRegistry struct {
	mu       sync.RWMutex
	commands map[string]slashcmd.SlashCommand
}

// NewCommandRegistry 创建新的命令注册表
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[string]slashcmd.SlashCommand),
	}
}

// Register 注册命令
// 如果同名命令已存在，会返回错误
func (r *CommandRegistry) Register(cmd slashcmd.SlashCommand) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := cmd.Name()

	if _, exists := r.commands[name]; exists {
		return fmt.Errorf("command already registered: /%s", name)
	}

	r.commands[name] = cmd
	return nil
}

// Get 获取命令
func (r *CommandRegistry) Get(name string) slashcmd.SlashCommand {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.commands[name]
}

// Has 检查命令是否存在
func (r *CommandRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.commands[name]
	return exists
}

// Names 返回所有命令名称（按字母排序）
func (r *CommandRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.commands))
	for name := range r.commands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// List 返回所有命令
func (r *CommandRegistry) List() []slashcmd.SlashCommand {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]slashcmd.SlashCommand, 0, len(r.commands))
	for _, cmd := range r.commands {
		result = append(result, cmd)
	}
	return result
}

// Count 返回命令数量
func (r *CommandRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.commands)
}