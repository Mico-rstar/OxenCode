package tools

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/yourname/oxencode/pkg/logger"
)

// LocalEnvironment 本地文件系统环境实现
// 这是 MVP 版本的默认环境实现，直接使用本地文件系统
type LocalEnvironment struct {
	basePath string         // 工作根目录（绝对路径）
	logger   logger.Logger // 封装的日志记录器
}

// NewLocalEnvironment 创建本地环境
// basePath 可以是相对路径或绝对路径，会被转换为绝对路径
// logger 是可选的，如果传入 nil 则创建基于全局 logger 的实例
func NewLocalEnvironment(basePath string, log logger.Logger) (*LocalEnvironment, error) {
	// 如果没有提供 logger，使用全局 logger 创建一个
	if log == nil {
		log = logger.New("env.local")
	} else {
		log = log.Named("env.local")
	}

	// 转换为绝对路径
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		log.Error("Failed to resolve absolute path",
			"path", basePath,
			"error", err)
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// 确保目录存在
	if err := os.MkdirAll(absPath, 0755); err != nil {
		log.Error("Failed to create working directory",
			"path", absPath,
			"error", err)
		return nil, fmt.Errorf("failed to create working directory: %w", err)
	}

	log.Info("Local environment created", "basePath", absPath)

	return &LocalEnvironment{
		basePath: absPath,
		logger:   log,
	}, nil
}

// GetWorkingDirectory 返回工作目录的绝对路径
func (e *LocalEnvironment) GetWorkingDirectory() string {
	return e.basePath
}

// ReadFile 读取文件内容
func (e *LocalEnvironment) ReadFile(path string) ([]byte, error) {
	fullPath := e.ResolvePath(path)
	e.logger.Debug("Reading file", "path", path, "fullPath", fullPath)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		e.logger.Error("Failed to read file", "path", fullPath, "error", err)
		return nil, err
	}

	return content, nil
}

// WriteFile 写入文件内容
func (e *LocalEnvironment) WriteFile(path string, data []byte, perm fs.FileMode) error {
	fullPath := e.ResolvePath(path)
	e.logger.Debug("Writing file",
		"path", path,
		"fullPath", fullPath,
		"size", len(data))

	// 确保父目录存在
	dir := filepath.Dir(fullPath)
	if dir != "" && dir != e.basePath {
		if err := os.MkdirAll(dir, 0755); err != nil {
			e.logger.Error("Failed to create parent directory",
				"dir", dir,
				"error", err)
			return fmt.Errorf("failed to create parent directory: %w", err)
		}
	}

	if err := os.WriteFile(fullPath, data, perm); err != nil {
		e.logger.Error("Failed to write file", "path", fullPath, "error", err)
		return err
	}

	return nil
}

// ListFiles 列出匹配模式的文件
func (e *LocalEnvironment) ListFiles(pattern string) ([]string, error) {
	fullPath := e.ResolvePath(pattern)
	e.logger.Debug("Listing files", "pattern", pattern, "fullPath", fullPath)

	matches, err := filepath.Glob(fullPath)
	if err != nil {
		e.logger.Error("Failed to list files", "pattern", fullPath, "error", err)
		return nil, err
	}

	return matches, nil
}

// FileExists 检查文件是否存在
func (e *LocalEnvironment) FileExists(path string) bool {
	fullPath := e.ResolvePath(path)
	_, err := os.Stat(fullPath)
	return err == nil
}

// ExecCommand 执行命令（在工作目录下）
func (e *LocalEnvironment) ExecCommand(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	e.logger.Debug("Executing command",
		"cmd", cmd,
		"args", args,
		"dir", e.basePath)

	c := exec.CommandContext(ctx, cmd, args...)
	c.Dir = e.basePath

	output, err := c.CombinedOutput()
	if err != nil {
		e.logger.Error("Command failed",
			"cmd", cmd,
			"error", err,
			"output", string(output))
		return output, fmt.Errorf("command failed: %w", err)
	}

	return output, nil
}

// ExecCommandWithWorkingDir 在指定目录执行命令
func (e *LocalEnvironment) ExecCommandWithWorkingDir(ctx context.Context, dir, cmd string, args ...string) ([]byte, error) {
	fullPath := e.ResolvePath(dir)
	e.logger.Debug("Executing command in directory",
		"cmd", cmd,
		"args", args,
		"dir", fullPath)

	c := exec.CommandContext(ctx, cmd, args...)
	c.Dir = fullPath

	output, err := c.CombinedOutput()
	if err != nil {
		e.logger.Error("Command failed",
			"cmd", cmd,
			"dir", fullPath,
			"error", err)
		return output, fmt.Errorf("command failed: %w", err)
	}

	return output, nil
}

// ResolvePath 解析相对路径为绝对路径
// 如果 path 已经是绝对路径，则直接返回
// 否则，将其与 basePath 拼接
func (e *LocalEnvironment) ResolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(e.basePath, path)
}

// Cleanup 清理资源
// 本地环境无需清理，返回 nil
func (e *LocalEnvironment) Cleanup() error {
	e.logger.Info("Cleaning up local environment", "basePath", e.basePath)
	// 本地环境无需清理
	return nil
}
