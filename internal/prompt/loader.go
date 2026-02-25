package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Loader 系统提示词加载器
type Loader struct {
	promptDir string
}

// NewLoader 创建新的提示词加载器
func NewLoader(promptDir string) *Loader {
	return &Loader{
		promptDir: promptDir,
	}
}

// Load 加载完整的系统提示词（解析 INCLUDE 指令）
func (l *Loader) Load() (string, error) {
	mainPath := filepath.Join(l.promptDir, "main_prompt.md")

	// 读取主提示词文件
	content, err := os.ReadFile(mainPath)
	if err != nil {
		return "", fmt.Errorf("failed to read main_prompt.md: %w", err)
	}

	// 解析并处理 INCLUDE 指令
	result, err := l.processIncludes(string(content))
	if err != nil {
		return "", fmt.Errorf("failed to process includes: %w", err)
	}

	return result, nil
}

// processIncludes 处理 {{INCLUDE:}} 指令
func (l *Loader) processIncludes(content string) (string, error) {
	lines := strings.Split(content, "\n")
	var result strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 检查是否是 INCLUDE 指令
		if strings.HasPrefix(trimmed, "{{INCLUDE:") && strings.HasSuffix(trimmed, "}}") {
			// 提取模块路径
			modulePath := strings.TrimPrefix(trimmed, "{{INCLUDE:")
			modulePath = strings.TrimSuffix(modulePath, "}}")
			modulePath = strings.TrimSpace(modulePath)

			// 读取模块内容
			moduleContent, err := l.loadModule(modulePath)
			if err != nil {
				return "", fmt.Errorf("failed to load module %s: %w", modulePath, err)
			}

			result.WriteString(moduleContent)
			result.WriteString("\n")
		} else {
			result.WriteString(line)
			result.WriteString("\n")
		}
	}

	return result.String(), nil
}

// loadModule 加载单个模块
func (l *Loader) loadModule(modulePath string) (string, error) {
	fullPath := filepath.Join(l.promptDir, modulePath)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// LoadRaw 直接加载主提示词文件（不解析 INCLUDE）
func (l *Loader) LoadRaw() (string, error) {
	mainPath := filepath.Join(l.promptDir, "main_prompt.md")

	content, err := os.ReadFile(mainPath)
	if err != nil {
		return "", fmt.Errorf("failed to read main_prompt.md: %w", err)
	}

	return string(content), nil
}

// SplitLines 将文本分割为行
func SplitLines(text string) []string {
	return strings.Split(text, "\n")
}
