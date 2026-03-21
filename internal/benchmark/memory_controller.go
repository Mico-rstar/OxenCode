package benchmark

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yourname/oxencode/pkg/config"
	"github.com/yourname/oxencode/pkg/memory"
)

// 默认配置
const (
	DefaultBenchmarkMemoryDir = ".oxencode/benchmark_memory"
	DefaultBenchmarkPort      = 8766
)

// MemoryController 记忆控制器接口
type MemoryController interface {
	// Reset 重置记忆状态
	Reset(ctx context.Context) error

	// InjectMemories 注入记忆
	InjectMemories(ctx context.Context, pre *MemoryPrecondition) error

	// GetState 获取当前记忆状态
	GetState(ctx context.Context) (*MemoryState, error)
}

// FileMemoryController 基于文件系统的记忆控制器
type FileMemoryController struct {
	memoryDir    string
	serviceURL   string
	memoryClient *memory.Client
}

// NewFileMemoryController 创建文件记忆控制器
func NewFileMemoryController(serviceURL string) (*FileMemoryController, error) {
	return NewFileMemoryControllerWithDir(serviceURL, "")
}

// NewFileMemoryControllerWithDir 创建文件记忆控制器（指定目录）
func NewFileMemoryControllerWithDir(serviceURL, memoryDir string) (*FileMemoryController, error) {
	// 使用隔离的 benchmark 记忆目录
	if memoryDir == "" {
		memoryDir = filepath.Join(os.Getenv("HOME"), DefaultBenchmarkMemoryDir)
	}

	// 确保目录存在
	subDirs := []string{"experience", "knowledge", "notes", "histories", "inner"}
	for _, subDir := range subDirs {
		path := filepath.Join(memoryDir, subDir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return nil, fmt.Errorf("创建目录失败 %s: %w", path, err)
		}
	}

	// 创建记忆客户端 (可选)
	var client *memory.Client
	if serviceURL != "" {
		clientCfg := &config.MemoryClientConfig{
			BaseURL: serviceURL,
			Timeout: 30 * time.Second,
		}
		client = memory.NewClient(clientCfg)
	}

	return &FileMemoryController{
		memoryDir:    memoryDir,
		serviceURL:   serviceURL,
		memoryClient: client,
	}, nil
}

// Reset 重置记忆状态
func (c *FileMemoryController) Reset(ctx context.Context) error {
	// 清空记忆目录内容（保留目录结构）
	subDirs := []string{"experience", "knowledge", "notes", "histories"}

	for _, subDir := range subDirs {
		dir := filepath.Join(c.memoryDir, subDir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // 忽略读取错误
		}

		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("删除文件失败 %s: %w", path, err)
			}
		}
	}

	// 重置 inner 文件
	innerDir := filepath.Join(c.memoryDir, "inner")
	selfPath := filepath.Join(innerDir, "self.md")
	userPath := filepath.Join(innerDir, "user.md")

	// 清空或删除
	os.Remove(selfPath)
	os.Remove(userPath)

	return nil
}

// InjectMemories 注入记忆
func (c *FileMemoryController) InjectMemories(ctx context.Context, pre *MemoryPrecondition) error {
	// 注入经验
	for _, entry := range pre.Experience {
		if err := c.injectMemoryFile("experience", &entry); err != nil {
			return fmt.Errorf("注入经验失败: %w", err)
		}
	}

	// 注入知识
	for _, entry := range pre.Knowledge {
		if err := c.injectMemoryFile("knowledge", &entry); err != nil {
			return fmt.Errorf("注入知识失败: %w", err)
		}
	}

	// 注入笔记
	for _, entry := range pre.Notes {
		if err := c.injectMemoryFile("notes", &entry); err != nil {
			return fmt.Errorf("注入笔记失败: %w", err)
		}
	}

	// 设置 inner 内容
	if pre.InnerSelf != "" {
		if err := c.setInnerFile("self.md", pre.InnerSelf); err != nil {
			return fmt.Errorf("设置 self.md 失败: %w", err)
		}
	}

	if pre.InnerUser != "" {
		if err := c.setInnerFile("user.md", pre.InnerUser); err != nil {
			return fmt.Errorf("设置 user.md 失败: %w", err)
		}
	}

	// 触发向量索引更新（如果有记忆服务）
	if c.memoryClient != nil {
		// 可以调用记忆服务的 re_embed 端点
		// 这里暂时跳过，因为需要服务端支持
	}

	return nil
}

// injectMemoryFile 注入单个记忆文件
func (c *FileMemoryController) injectMemoryFile(category string, entry *MemoryEntry) error {
	dir := filepath.Join(c.memoryDir, category)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 构建 markdown 内容
	var content strings.Builder

	// 写入 frontmatter
	content.WriteString("---\n")
	content.WriteString(fmt.Sprintf("description: %s\n", entry.Description))
	if len(entry.Metadata) > 0 {
		for k, v := range entry.Metadata {
			content.WriteString(fmt.Sprintf("%s: %s\n", k, v))
		}
	}
	content.WriteString("---\n\n")

	// 写入正文
	content.WriteString(entry.Content)

	// 写入文件
	filename := entry.ID + ".md"
	path := filepath.Join(dir, filename)

	return os.WriteFile(path, []byte(content.String()), 0644)
}

// setInnerFile 设置 inner 文件
func (c *FileMemoryController) setInnerFile(filename, content string) error {
	dir := filepath.Join(c.memoryDir, "inner")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, filename)
	return os.WriteFile(path, []byte(content), 0644)
}

// GetState 获取当前记忆状态
func (c *FileMemoryController) GetState(ctx context.Context) (*MemoryState, error) {
	state := &MemoryState{
		Files: make(map[string]string),
	}

	// 统计各类记忆数量
	state.ExperienceCount = c.countFiles("experience")
	state.KnowledgeCount = c.countFiles("knowledge")
	state.NotesCount = c.countFiles("notes")

	// 读取 inner 文件
	innerDir := filepath.Join(c.memoryDir, "inner")
	if data, err := os.ReadFile(filepath.Join(innerDir, "self.md")); err == nil {
		state.InnerSelf = string(data)
	}
	if data, err := os.ReadFile(filepath.Join(innerDir, "user.md")); err == nil {
		state.InnerUser = string(data)
	}

	return state, nil
}

// countFiles 统计目录中的文件数量
func (c *FileMemoryController) countFiles(subDir string) int {
	dir := filepath.Join(c.memoryDir, subDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			count++
		}
	}
	return count
}

// MockMemoryController 模拟记忆控制器（用于测试）
type MockMemoryController struct {
	state *MemoryState
}

// NewMockMemoryController 创建模拟记忆控制器
func NewMockMemoryController() *MockMemoryController {
	return &MockMemoryController{
		state: &MemoryState{
			Files: make(map[string]string),
		},
	}
}

// Reset 重置记忆状态
func (c *MockMemoryController) Reset(ctx context.Context) error {
	c.state = &MemoryState{
		Files: make(map[string]string),
	}
	return nil
}

// InjectMemories 注入记忆
func (c *MockMemoryController) InjectMemories(ctx context.Context, pre *MemoryPrecondition) error {
	c.state.ExperienceCount = len(pre.Experience)
	c.state.KnowledgeCount = len(pre.Knowledge)
	c.state.NotesCount = len(pre.Notes)
	c.state.InnerSelf = pre.InnerSelf
	c.state.InnerUser = pre.InnerUser
	return nil
}

// GetState 获取当前记忆状态
func (c *MockMemoryController) GetState(ctx context.Context) (*MemoryState, error) {
	return c.state, nil
}