package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yourname/oxencode/pkg/logger"
)

// GrepTool 内容搜索工具
type GrepTool struct {
	BaseTool
	env Environment
}

// NewGrepTool 创建 Grep 工具
func NewGrepTool(env Environment, log logger.Logger) *GrepTool {
	if log == nil {
		log = logger.New("tool.grep")
	} else {
		log = log.Named("tool.grep")
	}

	return &GrepTool{
		BaseTool: BaseTool{
			name:        "grep",
			description: "在文件中搜索匹配正则表达式的内容",
			parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"pattern": {
						"type": "string",
						"description": "正则表达式搜索模式"
					},
					"path": {
						"type": "string",
						"description": "搜索路径，默认为当前目录"
					},
					"file_pattern": {
						"type": "string",
						"description": "文件过滤模式（如 *.go）"
					},
					"ignore_case": {
						"type": "boolean",
						"description": "忽略大小写"
					}
				},
				"required": ["pattern"]
			}`),
			logger: log,
		},
		env: env,
	}
}

// Execute 执行 Grep 工具
func (t *GrepTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	pattern, ok := input["pattern"].(string)
	if !ok {
		return "", fmt.Errorf("pattern must be a string")
	}

	path := "."
	if p, ok := input["path"].(string); ok {
		path = p
	}

	ignoreCase := false
	if ic, ok := input["ignore_case"].(bool); ok {
		ignoreCase = ic
	}

	t.logger.Debug("Executing grep", "pattern", pattern, "path", path, "ignoreCase", ignoreCase)

	// 编译正则表达式
	if ignoreCase {
		pattern = "(?i)" + pattern
	}
	regex, err := regexp.Compile(pattern)
	if err != nil {
		t.logger.Error("Invalid regex", "error", err)
		return "", fmt.Errorf("invalid regex: %w", err)
	}

	// 收集文件（使用环境）
	var files []string
	if fp, ok := input["file_pattern"].(string); ok {
		files, _ = t.env.ListFiles(filepath.Join(path, fp))
	} else {
		files, _ = t.env.ListFiles(filepath.Join(path, "*"))
	}

	// 如果没有文件，尝试递归搜索
	if len(files) == 0 {
		files, _ = t.env.ListFiles(filepath.Join(path, "**"))
	}

	workDir := t.env.GetWorkingDirectory()

	// 搜索匹配
	var results []string
	matchCount := 0
	for _, file := range files {
		content, err := t.env.ReadFile(file)
		if err != nil {
			t.logger.Debug("Failed to read file", "file", file, "error", err)
			continue
		}

		relPath, err := filepath.Rel(workDir, file)
		if err != nil {
			relPath = file
		}

		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			if regex.MatchString(line) {
				results = append(results, fmt.Sprintf("%s:%d:%s", relPath, i+1, line))
				matchCount++
			}
		}
	}

	t.logger.Info("Grep completed", "matchCount", matchCount, "filesSearched", len(files))

	if len(results) == 0 {
		return "No matches found", nil
	}

	// 限制结果数量，避免输出过长
	const maxResults = 1000
	if len(results) > maxResults {
		results = results[:maxResults]
		return strings.Join(results, "\n") + fmt.Sprintf("\n... (%d more results truncated)", len(results)-maxResults), nil
	}

	return strings.Join(results, "\n"), nil
}
