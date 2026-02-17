package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yourname/oxencode/pkg/logger"
)

// GlobTool 文件路径模式匹配工具
type GlobTool struct {
	BaseTool
	env Environment
}

// NewGlobTool 创建 Glob 工具
func NewGlobTool(env Environment, log logger.Logger) *GlobTool {
	if log == nil {
		log = logger.New("tool.glob")
	} else {
		log = log.Named("tool.glob")
	}

	return &GlobTool{
		BaseTool: BaseTool{
			name:        "glob",
			description: "使用通配符模式查找文件",
			parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"pattern": {
						"type": "string",
						"description": "文件匹配模式（支持 *, **, ? 等通配符）"
					},
					"path": {
						"type": "string",
						"description": "搜索路径，默认为当前目录"
					}
				},
				"required": ["pattern"]
			}`),
			logger: log,
		},
		env: env,
	}
}

// Execute 执行 Glob 工具
func (t *GlobTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	pattern, ok := input["pattern"].(string)
	if !ok {
		return "", fmt.Errorf("pattern must be a string")
	}

	path := "."
	if p, ok := input["path"].(string); ok {
		path = p
	}

	t.logger.Debug("Executing glob", "pattern", pattern, "path", path)

	// 使用环境进行列表操作
	searchPattern := filepath.Join(path, pattern)
	matches, err := t.env.ListFiles(searchPattern)
	if err != nil {
		t.logger.Error("Glob failed", "error", err)
		return "", fmt.Errorf("glob failed: %w", err)
	}

	// 格式化输出（相对于工作目录）
	result := make([]string, 0, len(matches))
	workDir := t.env.GetWorkingDirectory()
	for _, m := range matches {
		relPath, err := filepath.Rel(workDir, m)
		if err != nil {
			// 如果无法转换为相对路径，使用绝对路径
			result = append(result, m)
		} else {
			result = append(result, relPath)
		}
	}

	t.logger.Info("Glob completed", "matchCount", len(result))

	if len(result) == 0 {
		return "No matches found", nil
	}

	return strings.Join(result, "\n"), nil
}
