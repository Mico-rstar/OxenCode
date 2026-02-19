package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yourname/oxencode/pkg/logger"
)

// BashTool 命令执行工具
type BashTool struct {
	BaseTool
	env         Environment
	defaultTimeout time.Duration
}

// NewBashTool 创建 Bash 工具
func NewBashTool(env Environment, log logger.Logger) *BashTool {
	if log == nil {
		log = logger.New("tool.bash")
	} else {
		log = log.Named("tool.bash")
	}

	return &BashTool{
		BaseTool: BaseTool{
			name:        "bash",
			description: "执行 shell 命令（支持异步执行和输出捕获）",
			parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"command": {
						"type": "string",
						"description": "要执行的命令"
					},
					"args": {
						"type": "array",
						"items": {"type": "string"},
						"description": "命令参数列表"
					},
					"dir": {
						"type": "string",
						"description": "执行命令的目录（相对路径），默认为工作目录"
					},
					"timeout": {
						"type": "integer",
						"description": "超时时间（秒），默认 120 秒"
					},
					"background": {
						"type": "boolean",
						"description": "是否在后台执行，默认为 false"
					}
				},
				"required": ["command"]
			}`),
			logger: log,
		},
		env:           env,
		defaultTimeout: 120 * time.Second,
	}
}

// Execute 执行 Bash 工具
func (t *BashTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	command, ok := input["command"].(string)
	if !ok {
		return "", fmt.Errorf("command must be a string")
	}

	// 获取参数 - 支持 []string 和 []any 两种类型
	var args []string
	if a, ok := input["args"].([]any); ok {
		args = make([]string, len(a))
		for i, arg := range a {
			args[i] = fmt.Sprintf("%v", arg)
		}
	} else if a, ok := input["args"].([]string); ok {
		// 直接使用 []string 类型
		args = a
	}

	// 获取工作目录
	workDir := t.env.GetWorkingDirectory()
	if dir, ok := input["dir"].(string); ok {
		workDir = t.env.ResolvePath(dir)
	}

	// 获取超时时间
	timeout := t.defaultTimeout
	if to, ok := input["timeout"].(float64); ok {
		timeout = time.Duration(to) * time.Second
	}

	// 检查是否后台执行
	background := false
	if bg, ok := input["background"].(bool); ok {
		background = bg
	}

	t.logger.Debug("Executing bash command",
		"command", command,
		"args", args,
		"dir", workDir,
		"timeout", timeout,
		"background", background)

	// 创建带超时的上下文
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 构建命令
	cmd := exec.CommandContext(cmdCtx, command, args...)
	cmd.Dir = workDir

	// 设置环境变量
	cmd.Env = os.Environ()

	if background {
		// 后台执行模式
		if err := cmd.Start(); err != nil {
			t.logger.Error("Failed to start background command", "error", err)
			return "", fmt.Errorf("failed to start background command: %w", err)
		}

		pid := cmd.Process.Pid
		t.logger.Info("Background command started", "pid", pid, "command", command)

		// 不等待命令完成
		return fmt.Sprintf("Background command started with PID: %d", pid), nil
	}

	// 前台执行模式，捕获输出
	output, err := cmd.CombinedOutput()
	exitCode := 0

	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	if err != nil {
		t.logger.Error("Command failed",
			"command", command,
			"exitCode", exitCode,
			"error", err,
			"output", string(output))

		// 返回输出和错误信息
		errorMsg := string(output)
		if errorMsg == "" {
			errorMsg = err.Error()
		}
		return fmt.Sprintf("Command failed with exit code %d:\n%s", exitCode, errorMsg), nil
	}

	t.logger.Info("Command completed successfully",
		"command", command,
		"exitCode", exitCode,
		"outputLength", len(output))

	if len(output) == 0 {
		return "Command completed successfully (no output)", nil
	}

	return string(output), nil
}

// ExecuteWithShell 在 shell 中执行命令字符串
// 这允许使用管道、重定向等 shell 特性
func (t *BashTool) ExecuteWithShell(ctx context.Context, commandStr string, dir string, timeout time.Duration) (string, error) {
	t.logger.Debug("Executing shell command string",
		"command", commandStr,
		"dir", dir,
		"timeout", timeout)

	// 根据操作系统选择 shell
	shell := "/bin/bash"
	shellFlag := "-c"
	if os.Getenv("OS") == "Windows_NT" || filepath.IsAbs("C:\\") {
		// Windows 环境
		shell = "cmd"
		shellFlag = "/C"
	}

	// 创建带超时的上下文
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, shell, shellFlag, commandStr)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.logger.Error("Shell command failed", "error", err)
		return "", fmt.Errorf("shell command failed: %w", err)
	}

	return string(output), nil
}

// ParseCommandString 解析命令字符串为命令和参数
// 支持简单的引号处理
func ParseCommandString(cmdStr string) (command string, args []string, err error) {
	// 简单实现：按空格分割，支持引号
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("empty command string")
	}

	command = parts[0]
	args = parts[1:]

	return command, args, nil
}
