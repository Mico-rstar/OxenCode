package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

// 正则表达式模式（包级别，编译一次）
var (
	// tagRegex 匹配 <tag> 格式的开始标签
	tagOpenRegex = regexp.MustCompile(`<([a-zA-Z_][a-zA-Z0-9_]*)>`)
	// varRegex 匹配 {{VAR:name}} 格式的变量占位符
	varRegex = regexp.MustCompile(`{{VAR:([a-zA-Z_][a-zA-Z0-9_]*)}}`)
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
		content, err := p.ReadFile(promptTag)
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
func (p *Prompt) ReadFile(relativePath string) (string, error) {
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
			moduleContent, err := p.ReadFile(modulePath)
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

// ExtractVariables 从内容中提取 XML 风格的标签对
// 提取 <tag>content</tag> 格式的标签，返回 map[tag]content
// 如果没有找到标签，返回空 map（非 nil）
// 重复标签：最后一次出现的值胜出
// 嵌套标签：只提取最外层标签
func ExtractVariables(content string) map[string]string {
	result := make(map[string]string)
	var consumedRanges []tagRange // 已消费的标签范围

	// 找到所有开始标签的位置
	openMatches := tagOpenRegex.FindAllStringSubmatchIndex(content, -1)

	// 从前向后处理
	for _, openMatch := range openMatches {
		if len(openMatch) < 4 {
			continue
		}

		// openMatch[0:2] 是整个匹配的起止位置
		// openMatch[2:4] 是第一个捕获组（标签名）的起止位置
		tagStart := openMatch[0]
		tagEnd := openMatch[1]
		tagName := content[openMatch[2]:openMatch[3]]
		closeTag := "</" + tagName + ">"

		// 检查此标签的起始位置是否已被其他标签消费（嵌套检查）
		if isPositionConsumed(tagStart, consumedRanges) {
			continue
		}

		// 从标签内容开始位置查找对应的结束标签
		contentStart := tagEnd
		closeTagPos := strings.Index(content[contentStart:], closeTag)

		if closeTagPos == -1 {
			// 没有找到结束标签，跳过
			continue
		}

		// 提取标签内容（在开始标签和结束标签之间）
		contentEnd := contentStart + closeTagPos
		tagContent := content[contentStart:contentEnd]

		// 存储结果（重复标签后者覆盖前者）
		result[tagName] = tagContent

		// 记录此标签的完整范围（包括开始和结束标签）
		fullEnd := contentEnd + len(closeTag)
		consumedRanges = append(consumedRanges, tagRange{
			start: tagStart,
			end:   fullEnd,
		})
	}

	return result
}

// tagRange 表示一个标签的字符范围
type tagRange struct {
	start int
	end   int
}

// isPositionConsumed 检查位置是否在任何已消费的范围内
func isPositionConsumed(pos int, ranges []tagRange) bool {
	for _, r := range ranges {
		if pos >= r.start && pos < r.end {
			return true
		}
	}
	return false
}

// InjectVariables 将 {{VAR:name}} 占位符替换为实际值
// 如果变量未找到，保留原始占位符（优雅降级）
func InjectVariables(content string, vars map[string]string) string {
	if vars == nil {
		return content
	}

	return varRegex.ReplaceAllStringFunc(content, func(match string) string {
		// 提取变量名
		submatches := varRegex.FindStringSubmatch(match)
		if len(submatches) >= 2 {
			varName := submatches[1]
			if val, ok := vars[varName]; ok {
				return val
			}
		}
		// 变量未找到，保留占位符
		return match
	})
}

// LoadWithVars 加载所有绑定的文件，并注入变量
// 处理顺序：读取文件 → 处理 {{INCLUDE:}} → 注入变量 → 设置字段值
// 保持向后兼容：现有的 Load() 方法不受影响
func (p *Prompt) LoadWithVars(vars map[string]string) error {
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
		content, err := p.ReadFile(promptTag)
		if err != nil {
			return fmt.Errorf("failed to load %s: %w", promptTag, err)
		}

		// 处理 {{INCLUDE:}} 指令
		content, err = p.processIncludes(content)
		if err != nil {
			return fmt.Errorf("failed to process includes for %s: %w", promptTag, err)
		}

		// 注入变量
		content = InjectVariables(content, vars)

		// 设置字段值
		v.Field(i).SetString(content)
	}

	p.loaded = true
	return nil
}
