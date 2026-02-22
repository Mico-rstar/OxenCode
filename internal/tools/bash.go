package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
	return NewBashToolWithTimeout(env, log, 120*time.Second)
}

// NewBashToolWithTimeout 创建带自定义超时的 Bash 工具
func NewBashToolWithTimeout(env Environment, log logger.Logger, timeout time.Duration) *BashTool {
	if log == nil {
		log = logger.New("tool.bash")
	} else {
		log = log.Named("tool.bash")
	}

	return &BashTool{
		BaseTool: BaseTool{
			name:        "bash",
			description: "执行 shell 命令（通过 /bin/sh -c 执行，支持管道、重定向、命令链等 shell 特性）",
			parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"command": {
						"type": "string",
						"description": "要执行的完整 shell 命令字符串"
					},
					"dir": {
						"type": "string",
						"description": "执行命令的目录（相对路径），默认为工作目录"
					},
					"timeout": {
						"type": "integer",
						"description": "超时时间（秒），默认使用配置文件中的 tool_timeout 值"
					}
				},
				"required": ["command"]
			}`),
			logger: log,
		},
		env:           env,
		defaultTimeout: timeout,
	}
}

// Execute 执行 Bash 工具
func (t *BashTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	command, ok := input["command"].(string)
	if !ok {
		return "", fmt.Errorf("command must be a string")
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

	t.logger.Debug("Executing bash command",
		"command", command,
		"dir", workDir,
		"timeout", timeout)

	// 创建带超时的上下文
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 使用 /bin/sh -c 执行命令，支持所有 shell 特性
	// shell := "/bin/bash"
	shell := "/bin/sh"
	shellFlag := "-c"

	cmd := exec.CommandContext(cmdCtx, shell, shellFlag, command)
	cmd.Dir = workDir

	// 设置环境变量
	cmd.Env = os.Environ()

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
