// Package slashcmd 定义斜杠命令的接口
// 此包独立于 cli 包，避免循环导入
package slashcmd

import "context"

// CommandContext 斜杠命令执行上下文
// 提供命令执行所需的接口
type CommandContext interface {
	// ClearSession 清空当前会话
	ClearSession()
	// NewSession 创建新会话
	NewSession() error
	// GetSessionID 获取当前会话 ID
	GetSessionID() string
}

// SlashCommand 斜杠命令接口
// 所有斜杠命令（如 /clear, /new）都实现此接口
type SlashCommand interface {
	// Name 返回命令名称（不含斜杠，如 "clear"）
	Name() string

	// Description 返回命令的简短描述
	Description() string

	// Execute 执行命令
	// ctx: 上下文
	// cmdCtx: 命令执行上下文，用于访问会话管理等资源
	// args: 命令参数（命令名称后的部分，已去除首尾空格）
	// 返回: 要显示给用户的输出，以及可能的错误
	Execute(ctx context.Context, cmdCtx CommandContext, args string) (string, error)
}

// CommandRegistryReader 命令注册表只读接口
type CommandRegistryReader interface {
	Names() []string
	Get(name string) SlashCommand
}