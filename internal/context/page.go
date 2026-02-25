package context

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/yourname/oxencode/internal/message"
)

// Page 维护一轮交互的所有 message
type Page struct {
	ID        PageID           `json:"id"`                // 页面唯一标识
	Type      PageType         `json:"type"`              // 页面类型 (L0/L1/L2)
	Strategy  *CompressionStrategy `json:"strategy"`      // 压缩策略配置
	Content   string           `json:"content"`           // 根据 schema 压缩后的内容缓存
	ArchivedFile string        `json:"archived_file"`     // 归档文件路径
	CreatedAt time.Time        `json:"created_at"`        // 创建时间
	UpdatedAt time.Time        `json:"updated_at"`        // 更新时间

	// 原始消息引用（L2 Pages 使用）
	Messages []message.Message `json:"messages,omitempty"`
}

// NewPage 创建新的 Page
func NewPage(pageType PageType, strategy *CompressionStrategy) *Page {
	return &Page{
		ID:        PageID(uuid.New().String()),
		Type:      pageType,
		Strategy:  strategy,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  make([]message.Message, 0),
	}
}

// NewL2Page 创建 L2 页面（原始消息页面）
func NewL2Page() *Page {
	_, _, l2Strategy := DefaultCompressionStrategies()
	return NewPage(PageTypeL2, l2Strategy)
}

// AddMessage 添加消息到 Page
func (p *Page) AddMessage(msg message.Message) {
	p.Messages = append(p.Messages, msg)
	p.UpdatedAt = time.Now()
}

// Compress 压缩页面内容
// 使用配置的 strategy 将原始 messages 压缩为 Content
func (p *Page) Compress(ctx context.Context, compressor Compressor) error {
	if p.Strategy == nil || p.Strategy.Schema == "" {
		// 没有配置压缩策略，直接序列化 messages
		data, err := json.Marshal(p.Messages)
		if err != nil {
			return fmt.Errorf("failed to marshal messages: %w", err)
		}
		p.Content = string(data)
		return nil
	}

	// 序列化原始消息
	// TODO: 改用其他方式序列化
	raw, err := json.Marshal(p.Messages)
	if err != nil {
		return fmt.Errorf("failed to marshal messages: %w", err)
	}

	// 使用压缩器进行压缩
	content, err := compressor.Compress(ctx, string(raw), p.Strategy)
	if err != nil {
		return fmt.Errorf("compression failed: %w", err)
	}

	p.Content = content
	p.UpdatedAt = time.Now()
	return nil
}

// Render 渲染页面内容为 fantasy.Message 格式
func (p *Page) Render() string {
	if p.Content != "" {
		return p.Content
	}
	// 如果没有压缩内容，返回原始消息的序列化
	data, _ := json.Marshal(p.Messages)
	return string(data)
}

// Archive 归档原始消息到文件系统
func (p *Page) Archive(archiveDir string) (string, error) {
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create archive directory: %w", err)
	}

	// 生成归档文件路径
	archiveFile := filepath.Join(archiveDir, fmt.Sprintf("%s.json", p.ID))

	// 序列化消息
	data, err := json.MarshalIndent(p.Messages, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal messages: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(archiveFile, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write archive file: %w", err)
	}

	p.ArchivedFile = archiveFile
	p.UpdatedAt = time.Now()
	return archiveFile, nil
}

// LoadFromArchive 从归档文件加载消息
func (p *Page) LoadFromArchive() error {
	if p.ArchivedFile == "" {
		return fmt.Errorf("no archived file path")
	}

	data, err := os.ReadFile(p.ArchivedFile)
	if err != nil {
		return fmt.Errorf("failed to read archive file: %w", err)
	}

	var messages []message.Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return fmt.Errorf("failed to unmarshal messages: %w", err)
	}

	p.Messages = messages
	return nil
}

// GetTokenCount 估算页面的 token 数量
func (p *Page) GetTokenCount() int {
	// 简单估算：每 4 个字符约 1 个 token
	if p.Content != "" {
		return len(p.Content) / 4
	}
	count := 0
	for _, msg := range p.Messages {
		count += len(msg.Content) / 4
	}
	return count
}

// IsCompressed 返回页面是否已压缩
func (p *Page) IsCompressed() bool {
	return p.Content != "" && p.Type != PageTypeL2
}
