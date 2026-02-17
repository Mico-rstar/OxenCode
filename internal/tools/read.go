package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yourname/oxencode/pkg/logger"
)

// ReadTool 文件读取工具，支持分页
type ReadTool struct {
	BaseTool
	env      Environment
	maxLines int // 最大读取行数
}

// NewReadTool 创建 Read 工具
func NewReadTool(env Environment, log logger.Logger) *ReadTool {
	if log == nil {
		log = logger.New("tool.read")
	} else {
		log = log.Named("tool.read")
	}

	return &ReadTool{
		BaseTool: BaseTool{
			name:        "read",
			description: "读取文件内容",
			parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file_path": {
						"type": "string",
						"description": "要读取的文件路径"
					},
					"offset": {
						"type": "integer",
						"description": "起始行号（从 1 开始），默认为 1"
					},
					"limit": {
						"type": "integer",
						"description": "读取行数，默认读取全部"
					}
				},
				"required": ["file_path"]
			}`),
			logger: log,
		},
		env:      env,
		maxLines: 10000,
	}
}

// Execute 执行 Read 工具
func (t *ReadTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	filePath, ok := input["file_path"].(string)
	if !ok {
		return "", fmt.Errorf("file_path must be a string")
	}

	offset := 0
	limit := -1

	if o, ok := input["offset"].(float64); ok {
		offset = int(o) - 1 // 转换为 0-based
	}
	if l, ok := input["limit"].(float64); ok {
		limit = int(l)
	}

	t.logger.Debug("Executing read", "filePath", filePath, "offset", offset, "limit", limit)

	// 使用环境读取文件
	content, err := t.env.ReadFile(filePath)
	if err != nil {
		t.logger.Error("Failed to read file", "file", filePath, "error", err)
		return "", fmt.Errorf("read failed: %w", err)
	}

	// 检查文件是否为空
	if len(content) == 0 {
		t.logger.Info("File is empty", "filePath", filePath)
		return "File is empty", nil
	}

	// 按行分割并应用偏移和限制
	lines := strings.Split(string(content), "\n")

	// 验证偏移量
	if offset < 0 {
		offset = 0
	}
	if offset > 0 && offset >= len(lines) {
		return "", fmt.Errorf("offset %d exceeds file length (%d lines)", offset+1, len(lines))
	}

	// 应用偏移
	if offset > 0 {
		lines = lines[offset:]
	}

	// 应用限制
	if limit > 0 && limit < len(lines) {
		lines = lines[:limit]
	}

	// 检查最大行数限制
	if len(lines) > t.maxLines {
		t.logger.Warn("File exceeds max lines, truncating", "lines", len(lines), "maxLines", t.maxLines)
		lines = lines[:t.maxLines]
	}

	// 添加行号
	var result []string
	startLine := offset + 1
	for i, line := range lines {
		result = append(result, fmt.Sprintf("%5d→%s", startLine+i, line))
	}

	t.logger.Info("Read completed", "lines", len(result), "filePath", filePath)

	if len(result) == 0 {
		return "File is empty", nil
	}

	return strings.Join(result, "\n"), nil
}
