package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// Prompt 系统提示词对象
// 通过 struct tag 绑定 prompts 目录的 md 文件
type Prompt struct {
	// 系统提示词
	SystemPrompt string `prompt:"main_prompt.md"`

	// 压缩器提示词
	CompressorSystemPrompt string `prompt:"compressor_system.md"`
	L0Schema               string `prompt:"l0_schema.md"`
	L1Schema               string `prompt:"l1_schema.md"`

	promptDir string
	cache     map[string]string
	loaded    bool
}

// New 创建新的 Prompt 对象
func New(promptDir string) *Prompt {
	return &Prompt{
		promptDir: promptDir,
		cache:     make(map[string]string),
	}
}

// load 加载所有绑定的文件，并自动处理 {{INCLUDE:}} 指令
func (p *Prompt) Load() error {
	if p.loaded {
		return nil
	}

	v := reflect.ValueOf(p).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		promptTag := field.Tag.Get("prompt")

		// 跳过没有 prompt tag 的字段
		if promptTag == "" {
			continue
		}

		// 从文件加载内容
		content, err := p.readFile(promptTag)
		if err != nil {
			return fmt.Errorf("failed to load %s: %w", promptTag, err)
		}

		// 处理 {{INCLUDE:}} 指令
		processedContent, err := p.processIncludes(content)
		if err != nil {
			return fmt.Errorf("failed to process includes for %s: %w", promptTag, err)
		}

		// 设置字段值
		v.Field(i).SetString(processedContent)
	}

	p.loaded = true
	return nil
}

// readFile 从 prompts 目录读取文件内容，使用缓存
func (p *Prompt) readFile(relativePath string) (string, error) {
	if content, ok := p.cache[relativePath]; ok {
		return content, nil
	}

	fullPath := filepath.Join(p.promptDir, relativePath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}

	content := string(data)
	p.cache[relativePath] = content
	return content, nil
}

// processIncludes 处理 {{INCLUDE:}} 指令
func (p *Prompt) processIncludes(content string) (string, error) {
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
			moduleContent, err := p.readFile(modulePath)
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

// SplitLines 将文本分割为行
func SplitLines(text string) []string {
	return strings.Split(text, "\n")
}
