package tools

import (
	"context"
	"io/fs"
)

// Environment 工具执行环境接口
// 该接口抽象了工具执行的底层环境，使得工具可以在不同的环境中运行
// MVP 版本使用本地文件系统，未来可扩展到容器、远程环境等
type Environment interface {
	// GetWorkingDirectory 获取当前工作目录
	GetWorkingDirectory() string

	// ReadFile 读取文件内容
	ReadFile(path string) ([]byte, error)

	// WriteFile 写入文件内容
	WriteFile(path string, data []byte, perm fs.FileMode) error

	// ListFiles 列出文件（支持通配符模式）
	ListFiles(pattern string) ([]string, error)

	// FileExists 检查文件是否存在
	FileExists(path string) bool

	// ExecCommand 执行命令
	ExecCommand(ctx context.Context, cmd string, args ...string) ([]byte, error)

	// ExecCommandWithWorkingDir 在指定目录执行命令
	ExecCommandWithWorkingDir(ctx context.Context, dir, cmd string, args ...string) ([]byte, error)

	// ResolvePath 解析相对路径为绝对路径
	ResolvePath(path string) string

	// Cleanup 清理资源（环境销毁时调用）
	Cleanup() error
}
