package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/yourname/oxencode/pkg/logger"
)

// WriteTool 文件写入工具
type WriteTool struct {
	BaseTool
	env Environment
}

// NewWriteTool 创建 Write 工具
func NewWriteTool(env Environment, log logger.Logger) *WriteTool {
	if log == nil {
		log = logger.New("tool.write")
	} else {
		log = log.Named("tool.write")
	}

	return &WriteTool{
		BaseTool: BaseTool{
			name:        "write",
			description: "写入文件内容（支持创建新文件和覆盖模式）",
			parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file_path": {
						"type": "string",
						"description": "要写入的文件路径（绝对路径或相对于工作目录的路径）"
					},
					"content": {
						"type": "string",
						"description": "要写入的内容"
					},
					"mode": {
						"type": "string",
						"enum": ["overwrite", "create"],
						"description": "写入模式：overwrite-覆盖已存在文件，create-仅创建新文件（如果文件已存在则报错）"
					},
					"perm": {
						"type": "string",
						"description": "文件权限（如 0644），默认为 0644"
					}
				},
				"required": ["file_path", "content"]
			}`),
			logger: log,
		},
		env: env,
	}
}

// Execute 执行 Write 工具
func (t *WriteTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	filePath, ok := input["file_path"].(string)
	if !ok {
		return "", fmt.Errorf("file_path must be a string")
	}

	content, ok := input["content"].(string)
	if !ok {
		return "", fmt.Errorf("content must be a string")
	}

	// 解析模式（默认为 overwrite）
	mode := "overwrite"
	if m, ok := input["mode"].(string); ok {
		mode = m
	}

	// 解析文件权限（默认为 0644）
	perm := fs.FileMode(0644)
	if p, ok := input["perm"].(string); ok {
		var parsedPerm uint32
		_, err := fmt.Sscanf(p, "%o", &parsedPerm)
		if err == nil {
			perm = fs.FileMode(parsedPerm)
		}
	}

	t.logger.Debug("Executing write",
		"filePath", filePath,
		"mode", mode,
		"perm", perm,
		"contentLength", len(content))

	// 检查文件是否已存在
	fileExists := t.env.FileExists(filePath)

	if mode == "create" && fileExists {
		return "", fmt.Errorf("file already exists and mode is 'create': %s", filePath)
	}

	// 写入文件
	data := []byte(content)
	if err := t.env.WriteFile(filePath, data, perm); err != nil {
		t.logger.Error("Failed to write file", "file", filePath, "error", err)
		return "", fmt.Errorf("write failed: %w", err)
	}

	action := "created"
	if fileExists {
		action = "overwritten"
	}

	t.logger.Info("File written successfully",
		"file", filePath,
		"action", action,
		"size", len(content))

	return fmt.Sprintf("File %s successfully (%d bytes written)", action, len(content)), nil
}

// Validate 验证输入参数
func (t *WriteTool) Validate(input map[string]any) error {
	// 先调用基础验证
	if err := t.BaseTool.Validate(input); err != nil {
		return err
	}

	// 验证模式
	if mode, ok := input["mode"].(string); ok {
		if mode != "overwrite" && mode != "create" {
			return &ValidationError{
				Field:   "mode",
				Message: "mode must be either 'overwrite' or 'create'",
			}
		}
	}

	// 验证文件路径不为空
	if filePath, ok := input["file_path"].(string); ok {
		if strings.TrimSpace(filePath) == "" {
			return &ValidationError{
				Field:   "file_path",
				Message: "file_path cannot be empty",
			}
		}

		// 检查路径是否尝试穿越工作目录
		fullPath := t.env.ResolvePath(filePath)
		workDir := t.env.GetWorkingDirectory()

		// 确保路径在工作目录内（安全检查）
		rel, err := filepath.Rel(workDir, fullPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			t.logger.Warn("Path may be outside working directory",
				"path", filePath,
				"fullPath", fullPath,
				"workDir", workDir)
		}
	}

	return nil
}
