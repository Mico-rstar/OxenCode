package context

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yourname/oxencode/internal/message"
)

// TruncatedMarker 截断标记
const TruncatedMarker = "\n[...truncated]"

// Page 维护一轮交互的所有 message
type Page struct {
	ID           PageID               `json:"id"`                // 页面唯一标识
	Type         PageType             `json:"type"`              // 页面类型 (L0/L1/L2)
	Strategy     *CompressionStrategy `json:"strategy"`         // 压缩策略配置
	Content      string               `json:"content"`           // 根据 schema 压缩后的内容缓存
	ArchivedFile string               `json:"archived_file"`     // 归档文件路径
	CreatedAt    time.Time            `json:"created_at"`        // 创建时间
	UpdatedAt    time.Time            `json:"updated_at"`        // 更新时间

	// 原子消息序列列表（替代原来的 Messages）
	// 每个 AtomSequence 是不可分割的单元，保证 tool_calls 和 tool results 的原子性
	Atoms []*message.AtomSequence `json:"atoms,omitempty"`

	// 原始消息引用（保留用于序列化/反序列化兼容性，以及 L0 渲染）
	Messages []message.Message `json:"messages,omitempty"`

	// 预处理后的消息（L1 使用）
	ProcessedMessages []message.Message `json:"processed_messages,omitempty"`
}

// NewPage 创建新的 Page
func NewPage(pageType PageType, strategy *CompressionStrategy) *Page {
	return &Page{
		ID:        PageID(uuid.New().String()),
		Type:      pageType,
		Strategy:  strategy,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Atoms:     make([]*message.AtomSequence, 0),
		Messages:  make([]message.Message, 0),
	}
}

// NewL2Page 创建 L2 页面（原始消息页面）
func NewL2Page() *Page {
	_, _, l2Strategy := DefaultCompressionStrategies()
	return NewPage(PageTypeL2, l2Strategy)
}

// AddAtom 添加原子消息序列到 Page
// 保证 Page 内部不会有孤儿消息（tool result 一定有对应的 assistant + tool_calls）
func (p *Page) AddAtom(atom *message.AtomSequence) {
	p.Atoms = append(p.Atoms, atom)
	p.UpdatedAt = time.Now()
}

// AddMessage 添加消息到 Page（保留向后兼容）
// 注意：此方法会创建一个单消息的 AtomSequence，仅适用于 user 消息或不含 tool_calls 的 assistant 消息
func (p *Page) AddMessage(msg message.Message) {
	atom := message.NewAtomSequence()
	if msg.Role == message.RoleAssistant {
		atom.SetAssistant(msg)
	} else if msg.Role == message.RoleUser {
		atom.SetUserMessage(msg)
	} else if msg.Role == message.RoleTool {
		// Tool 消息应该通过 AddAtom 添加，这里仅作为 fallback
		p.Messages = append(p.Messages, msg)
		p.UpdatedAt = time.Now()
		return
	}
	p.AddAtom(atom)
}

// BuildMessages 从 Atoms 构建消息列表
func (p *Page) BuildMessages() []message.Message {
	// 优先使用 Atoms 构建
	if len(p.Atoms) > 0 {
		msgs := make([]message.Message, 0)
		for _, atom := range p.Atoms {
			msgs = append(msgs, atom.ToMessages()...)
		}
		return msgs
	}
	// fallback 到原始 Messages
	return p.Messages
}

// Compress 压缩页面内容
// 使用配置的 strategy 将原始 messages 压缩为 Content
func (p *Page) Compress(ctx context.Context, compressor Compressor) error {
	if p.Strategy == nil || p.Strategy.Skill == "" {
		// 没有配置压缩策略，直接序列化 messages
		data, err := json.Marshal(p.Messages)
		if err != nil {
			return fmt.Errorf("failed to marshal messages: %w", err)
		}
		p.Content = string(data)
		return nil
	}

	// 序列化原始消息
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

// GetTokenCount 估算页面的 token 数量
func (p *Page) GetTokenCount() int {
	// 简单估算：每 4 个字符约 1 个 token
	if p.Content != "" {
		return len(p.Content) / 4
	}
	// 优先使用 Atoms 计算
	if len(p.Atoms) > 0 {
		count := 0
		for _, atom := range p.Atoms {
			count += atom.GetTokenCount()
		}
		return count
	}
	// fallback 到 Messages
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

// Preprocess 预处理消息，根据Strategy配置截断
// L1级别会对工具输出和Assistant消息进行截断
func (p *Page) Preprocess() {
	if p.Strategy == nil {
		return
	}

	// 从 Atoms 构建消息列表
	p.Messages = p.BuildMessages()

	processed := make([]message.Message, len(p.Messages))
	for i, msg := range p.Messages {
		processed[i] = p.truncateMessage(msg)
	}
	p.ProcessedMessages = processed
	p.UpdatedAt = time.Now()
}

// truncateMessage 根据策略截断消息
func (p *Page) truncateMessage(msg message.Message) message.Message {
	result := msg // 复制消息

	// 截断Assistant消息内容
	if msg.Role == message.RoleAssistant && p.Strategy.MaxAssistantLength > 0 {
		if len(msg.Content) > p.Strategy.MaxAssistantLength {
			result.Content = msg.Content[:p.Strategy.MaxAssistantLength] + TruncatedMarker
		}
	}

	// 截断工具输出（Assistant消息的ReActLoop中）
	if msg.Role == message.RoleAssistant && p.Strategy.MaxToolOutputLength > 0 {
		for j, step := range result.ReActLoop {
			if step.ToolCall != nil && len(step.ToolCall.Output) > p.Strategy.MaxToolOutputLength {
				result.ReActLoop[j].ToolCall.Output =
					step.ToolCall.Output[:p.Strategy.MaxToolOutputLength] + TruncatedMarker
			}
		}
	}

	// 截断Tool消息
	if msg.Role == message.RoleTool && p.Strategy.MaxToolOutputLength > 0 {
		if len(msg.Content) > p.Strategy.MaxToolOutputLength {
			result.Content = msg.Content[:p.Strategy.MaxToolOutputLength] + TruncatedMarker
		}
	}

	return result
}

// Render 渲染页面内容为可读文本
// L0使用压缩后的Content，L1/L2使用消息渲染（带角色标记）
func (p *Page) Render() string {
	// L0使用压缩后的Content
	if p.Type == PageTypeL0 && p.Content != "" {
		return p.Content
	}

	// L1/L2使用消息渲染
	messages := p.Messages
	if p.ProcessedMessages != nil {
		messages = p.ProcessedMessages
	}

	var sb strings.Builder
	for _, msg := range messages {
		switch msg.Role {
		case message.RoleUser:
			sb.WriteString("[User]\n")
			sb.WriteString(msg.Content)
			sb.WriteString("\n\n")
		case message.RoleAssistant:
			sb.WriteString("[Assistant]\n")
			sb.WriteString(msg.Content)
			sb.WriteString("\n\n")
			// 渲染工具调用
			for _, step := range msg.ReActLoop {
				if step.ToolCall != nil {
					sb.WriteString(fmt.Sprintf("[Tool: %s]\n", step.ToolCall.Name))
					sb.WriteString(step.ToolCall.Output)
					sb.WriteString("\n\n")
				}
			}
		case message.RoleTool:
			sb.WriteString("[Tool Result]\n")
			sb.WriteString(msg.Content)
			sb.WriteString("\n\n")
		}
	}
	return sb.String()
}
