package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/yourname/oxencode/pkg/logger"
)

// EditTool 文件编辑工具
type EditTool struct {
	BaseTool
	env Environment
}

// NewEditTool 创建 Edit 工具
func NewEditTool(env Environment, log logger.Logger) *EditTool {
	if log == nil {
		log = logger.New("tool.edit")
	} else {
		log = log.Named("tool.edit")
	}

	return &EditTool{
		BaseTool: BaseTool{
			name:        "edit",
			description: "编辑文件内容（支持字符串替换，可替换多处）",
			parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file_path": {
						"type": "string",
						"description": "要编辑的文件路径"
					},
					"old_string": {
						"type": "string",
						"description": "要替换的旧字符串"
					},
					"new_string": {
						"type": "string",
						"description": "替换后的新字符串"
					},
					"replace_all": {
						"type": "boolean",
						"description": "是否替换所有匹配项，默认为 false（仅替换第一个匹配项）"
					},
					"replacements": {
						"type": "array",
						"items": {
							"type": "object",
							"properties": {
								"old_string": {"type": "string"},
								"new_string": {"type": "string"}
							},
							"required": ["old_string", "new_string"]
						},
						"description": "批量替换列表（与 old_string/new_string 互斥）"
					},
					"create_backup": {
						"type": "boolean",
						"description": "是否在编辑前创建备份文件"
					}
				},
				"required": ["file_path"]
			}`),
			logger: log,
		},
		env: env,
	}
}

// Execute 执行 Edit 工具
func (t *EditTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	filePath, ok := input["file_path"].(string)
	if !ok {
		return "", fmt.Errorf("file_path must be a string")
	}

	// 检查文件是否存在
	if !t.env.FileExists(filePath) {
		return "", fmt.Errorf("file does not exist: %s", filePath)
	}

	// 读取文件内容
	content, err := t.env.ReadFile(filePath)
	if err != nil {
		t.logger.Error("Failed to read file", "file", filePath, "error", err)
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	originalContent := string(content)

	// 创建备份（如果需要）
	createBackup := false
	if cb, ok := input["create_backup"].(bool); ok {
		createBackup = cb
	}
	if createBackup {
		backupPath := filePath + ".bak"
		if err := t.env.WriteFile(backupPath, content, 0644); err != nil {
			t.logger.Warn("Failed to create backup", "backupPath", backupPath, "error", err)
		} else {
			t.logger.Info("Backup created", "backupPath", backupPath)
		}
	}

	var newContent string
	var replacementsMade int

	// 检查是否使用批量替换
	if replacements, ok := input["replacements"].([]any); ok && len(replacements) > 0 {
		// 批量替换模式
		newContent, replacementsMade, err = t.applyBatchReplacements(originalContent, replacements)
		if err != nil {
			return "", err
		}
	} else {
		// 单次替换模式
		oldStr, ok1 := input["old_string"].(string)
		newStr, ok2 := input["new_string"].(string)

		if !ok1 || !ok2 {
			return "", fmt.Errorf("old_string and new_string are required when not using replacements array")
		}

		if oldStr == "" {
			return "", fmt.Errorf("old_string cannot be empty")
		}

		replaceAll := false
		if ra, ok := input["replace_all"].(bool); ok {
			replaceAll = ra
		}

		newContent, replacementsMade = t.applySingleReplacement(originalContent, oldStr, newStr, replaceAll)
	}

	// 检查是否有修改
	if newContent == originalContent {
		t.logger.Info("No changes made", "file", filePath)
		return "No changes made - old_string not found in file", nil
	}

	// 写入修改后的内容
	if err := t.env.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		t.logger.Error("Failed to write file", "file", filePath, "error", err)
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	t.logger.Info("File edited successfully",
		"file", filePath,
		"replacements", replacementsMade,
		"sizeDiff", len(newContent)-len(originalContent))

	return fmt.Sprintf("File edited successfully (%d replacement(s) made)", replacementsMade), nil
}

// applySingleReplacement 应用单次替换
func (t *EditTool) applySingleReplacement(content, oldStr, newStr string, replaceAll bool) (string, int) {
	if replaceAll {
		count := strings.Count(content, oldStr)
		newContent := strings.ReplaceAll(content, oldStr, newStr)
		return newContent, count
	}

	// 仅替换第一个匹配项
	if strings.Contains(content, oldStr) {
		newContent := strings.Replace(content, oldStr, newStr, 1)
		return newContent, 1
	}

	return content, 0
}

// applyBatchReplacements 应用批量替换
func (t *EditTool) applyBatchReplacements(content string, replacements []any) (string, int, error) {
	result := content
	totalReplacements := 0

	for i, repl := range replacements {
		replMap, ok := repl.(map[string]any)
		if !ok {
			return "", 0, fmt.Errorf("replacement at index %d is not a valid object", i)
		}

		oldStr, ok1 := replMap["old_string"].(string)
		newStr, ok2 := replMap["new_string"].(string)

		if !ok1 || !ok2 {
			return "", 0, fmt.Errorf("replacement at index %d must have old_string and new_string", i)
		}

		if oldStr == "" {
			t.logger.Warn("Skipping empty old_string in batch replacement", "index", i)
			continue
		}

		// 检查是否有 replace_all 选项
		replaceAll := false
		if ra, ok := replMap["replace_all"].(bool); ok {
			replaceAll = ra
		}

		var newContent string
		var count int

		if replaceAll {
			count = strings.Count(result, oldStr)
			newContent = strings.ReplaceAll(result, oldStr, newStr)
		} else {
			if strings.Contains(result, oldStr) {
				newContent = strings.Replace(result, oldStr, newStr, 1)
				count = 1
			} else {
				newContent = result
				count = 0
			}
		}

		result = newContent
		totalReplacements += count
	}

	return result, totalReplacements, nil
}

// Validate 验证输入参数
func (t *EditTool) Validate(input map[string]any) error {
	// 先调用基础验证
	if err := t.BaseTool.Validate(input); err != nil {
		return err
	}

	// 验证 file_path
	filePath, ok := input["file_path"].(string)
	if !ok || strings.TrimSpace(filePath) == "" {
		return &ValidationError{
			Field:   "file_path",
			Message: "file_path is required and cannot be empty",
		}
	}

	// 检查是否提供了替换参数
	_, hasOldStr := input["old_string"]
	_, hasNewStr := input["new_string"]
	_, hasReplacements := input["replacements"]

	if !hasOldStr && !hasNewStr && !hasReplacements {
		return &ValidationError{
			Field:   "",
			Message: "either (old_string and new_string) or replacements array is required",
		}
	}

	// 如果使用单次替换模式，确保 old_string 和 new_string 都存在
	if !hasReplacements && (!hasOldStr || !hasNewStr) {
		return &ValidationError{
			Field:   "",
			Message: "both old_string and new_string are required when not using replacements array",
		}
	}

	return nil
}

// CreateBackupWithTimestamp 创建带时间戳的备份文件
func (t *EditTool) CreateBackupWithTimestamp(filePath string) error {
	content, err := t.env.ReadFile(filePath)
	if err != nil {
		return err
	}

	// 生成时间戳后缀
	timestamp := os.Getenv("BACKUP_TIMESTAMP")
	if timestamp == "" {
		timestamp = "backup"
	}

	backupPath := filePath + "." + timestamp + ".bak"
	return t.env.WriteFile(backupPath, content, 0644)
}

// PreviewReplacement 预览替换结果（不实际修改文件）
func (t *EditTool) PreviewReplacement(filePath, oldStr, newStr string, replaceAll bool) (string, int, error) {
	if !t.env.FileExists(filePath) {
		return "", 0, fmt.Errorf("file does not exist: %s", filePath)
	}

	content, err := t.env.ReadFile(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read file: %w", err)
	}

	originalContent := string(content)
	newContent, count := t.applySingleReplacement(originalContent, oldStr, newStr, replaceAll)

	return newContent, count, nil
}
