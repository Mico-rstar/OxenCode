package context

import (
	"context"
	"sync"
	"time"

	"github.com/yourname/oxencode/internal/message"
	"github.com/yourname/oxencode/pkg/config"
	"github.com/yourname/oxencode/pkg/logger"
)

// Session 上下文会话，管理完整的上下文窗口
type Session struct {
	ID string `json:"id"`

	// 上下文窗口 (system -> L0 -> L1 -> L2)
	SystemPrompt string   `json:"system_prompt"` // 系统 Prompt
	L0Page       *Page    `json:"l0_page"`       // 全局唯一的 L0 Page
	L1Pages      []*Page  `json:"l1_pages"`      // L1 Pages 列表（按时间倒序，最新的在前）
	L2Pages      []*Page  `json:"l2_pages"`      // L2 Pages 列表（按时间倒序，最新的在前）

	// 配置
	MaxL1Pages int `json:"max_l1_pages"` // L1 Page 最大数量
	cfg        *config.Config `json:"-"` // 配置引用

	// 压缩策略
	L0Strategy *CompressionStrategy `json:"l0_strategy"`
	L1Strategy *CompressionStrategy `json:"l1_strategy"`
	L2Strategy *CompressionStrategy `json:"l2_strategy"`

	// 异步压缩管理（仅用于L0）
	compressor  Compressor   `json:"-"`  // 压缩器
	compressWkr *CompressWorker `json:"-"` // 压缩工作器

	// 归档目录
	ArchiveDir string `json:"archive_dir"`

	// 同步锁
	mu sync.RWMutex `json:"-"`

	// 日志
	logger logger.Logger `json:"-"`

	// 状态
	initialized bool `json:"-"`
}

// SessionConfig Session 配置
type SessionConfig struct {
	SystemPrompt string
	MaxL1Pages   int
	ArchiveDir   string
	Compressor   Compressor
	Cfg          *config.Config // 配置引用
}

// DefaultSessionConfig 返回默认的 Session 配置
func DefaultSessionConfig() *SessionConfig {
	return &SessionConfig{
		SystemPrompt: "You are a helpful AI programming assistant.",
		MaxL1Pages:   10, // 默认保留 10 个 L1 pages
		ArchiveDir:   "", // 使用默认归档目录
		Compressor:   nil, // 需要外部设置
		Cfg:          nil, // 需要外部设置
	}
}

// NewSession 创建新的 Session
func NewSession(sessionConfig *SessionConfig) (*Session, error) {
	log := logger.New("context/session")

	if sessionConfig.Compressor == nil {
		panic("Compressor is required, cannot create Session without it")
	}

	cfg := sessionConfig.Cfg
	if cfg == nil {
		cfg = config.Get()
	}

	// 从Config创建策略
	var l0Strategy, l1Strategy, l2Strategy *CompressionStrategy
	if cfg != nil {
		l0Strategy = NewCompressionStrategy(PageTypeL0, cfg)
		l1Strategy = NewCompressionStrategy(PageTypeL1, cfg)
		l2Strategy = NewCompressionStrategy(PageTypeL2, cfg)
	} else {
		l0Strategy, l1Strategy, l2Strategy = DefaultCompressionStrategies()
	}

	session := &Session{
		ID:           time.Now().Format("20060102-150405"),
		SystemPrompt: sessionConfig.SystemPrompt,
		L0Page:       nil, // 初始时没有 L0 page
		L1Pages:      make([]*Page, 0),
		L2Pages:      make([]*Page, 0),
		MaxL1Pages:   sessionConfig.MaxL1Pages,
		cfg:          cfg,
		L0Strategy:   l0Strategy,
		L1Strategy:   l1Strategy,
		L2Strategy:   l2Strategy,
		compressor:   sessionConfig.Compressor,
		ArchiveDir:   sessionConfig.ArchiveDir,
		logger:       log,
		initialized:  true,
	}

	// 设置默认归档目录
	if session.ArchiveDir == "" {
		session.ArchiveDir = "~/.local/share/oxencode/archive"
	}

	// 创建并启动压缩工作器（仅用于L0）
	workerConfig := DefaultCompressWorkerConfig()
	session.compressWkr = NewCompressWorker(sessionConfig.Compressor, workerConfig)
	go session.processCompressResults()

	session.logger.Info("Session created", "id", session.ID, "max_l1_pages", session.MaxL1Pages)
	return session, nil
}

// AddMessage 添加消息到 Session（添加到 L2 Page）
func (s *Session) AddMessage(msg message.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 确保有当前的 L2 page
	if len(s.L2Pages) == 0 {
		s.L2Pages = append(s.L2Pages, NewL2Page())
	}

	// 添加到最新的 L2 page
	currentL2 := s.L2Pages[0]
	currentL2.AddMessage(msg)

	// 检查是否超过token限制（兜底策略）
	if s.cfg != nil && s.cfg.MaxPageTokens > 0 {
		if currentL2.GetTokenCount() > s.cfg.MaxPageTokens {
			s.splitL2PageLocked()
		}
	}

	s.logger.Debug("Message added", "page_id", currentL2.ID, "role", msg.Role)
}

// splitL2PageLocked 分页：旧消息→L1，新消息→L2（需要持有锁）
func (s *Session) splitL2PageLocked() {
	if len(s.L2Pages) == 0 {
		return
	}

	currentL2 := s.L2Pages[0]
	messages := currentL2.Messages

	if len(messages) < 2 {
		s.logger.Warn("Cannot split page with less than 2 messages")
		return
	}

	// 按1/2分割
	mid := len(messages) / 2
	oldMessages := messages[:mid]
	newMessages := messages[mid:]

	// 旧消息创建L1 Page
	l1Page := NewPage(PageTypeL1, s.L1Strategy)
	l1Page.Messages = oldMessages
	l1Page.Preprocess() // L1预处理（截断）
	s.L1Pages = append([]*Page{l1Page}, s.L1Pages...)

	// 新消息保留为L2
	newL2 := NewL2Page()
	newL2.Messages = newMessages
	s.L2Pages[0] = newL2

	s.logger.Info("L2 page split due to token limit",
		"old_messages", len(oldMessages),
		"new_messages", len(newMessages))
}

// Commit 提交当前 L2 page，创建 L1 page 并预处理
// L1不再调用LLM压缩，只进行截断预处理
func (s *Session) Commit(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.L2Pages) == 0 {
		return nil // 没有需要提交的
	}

	// 获取当前的 L2 page
	currentL2 := s.L2Pages[0]

	// 归档 L2 page
	if _, err := currentL2.Archive(s.ArchiveDir); err != nil {
		s.logger.Warn("Failed to archive L2 page", "error", err)
	}

	// 创建新的 L1 page
	l1Page := NewPage(PageTypeL1, s.L1Strategy)
	l1Page.Messages = currentL2.Messages

	// L1预处理（截断，不调用LLM）
	l1Page.Preprocess()

	// 添加到 L1Pages
	s.L1Pages = append([]*Page{l1Page}, s.L1Pages...)

	// 创建新的 L2 page 用于收集新消息
	s.L2Pages = append([]*Page{NewL2Page()}, s.L2Pages...)

	s.logger.Info("Session committed", "l2_page_id", currentL2.ID, "l1_page_id", l1Page.ID)
	return nil
}

// GetContext 构建当前的上下文窗口
// 返回格式化的消息列表，用于发送给 LLM
func (s *Session) GetContext() []message.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	messages := make([]message.Message, 0)

	// 1. 添加系统提示
	if s.SystemPrompt != "" {
		messages = append(messages, message.NewMessage(message.RoleSystem, s.SystemPrompt))
	}

	// 2. 添加 L0 page（如果有）
	if s.L0Page != nil {
		messages = append(messages, message.NewMessage(message.RoleSystem, s.L0Page.Render()))
	}

	// 3. 添加 L1 pages
	for _, p := range s.L1Pages {
		content := p.Render()
		messages = append(messages, message.NewMessage(message.RoleAssistant, content))
	}

	// 4. 添加 L2 pages（原始消息）
	for _, p := range s.L2Pages {
		messages = append(messages, p.Messages...)
	}

	s.logger.Debug("Context built", "total_messages", len(messages), "l1_count", len(s.L1Pages), "l2_count", len(s.L2Pages))
	return messages
}

// processCompressResults 处理压缩结果
func (s *Session) processCompressResults() {
	for result := range s.compressWkr.Results() {
		if result.Error != nil {
			s.logger.Error("Compression failed", "page_id", result.PageID, "error", result.Error)
			continue
		}

		s.logger.Info("Compress result received", "page_id", result.PageID)

		// 查找并更新对应的 page
		s.mu.Lock()
		var page *Page
		for i := range s.L1Pages {
			if s.L1Pages[i].ID == result.PageID {
				page = s.L1Pages[i]
				break
			}
		}

		if page != nil {
			page.Content = result.Content
			// 检查是否需要压缩 L0
			if len(s.L1Pages) > s.MaxL1Pages {
				s.compressL0Locked()
			}
			s.logger.Info("Page processed", "page_id", result.PageID)
		} else {
			s.logger.Warn("Page not found", "page_id", result.PageID)
		}
		s.mu.Unlock()
	}
}

// compressL0Locked 压缩 L0 page（需要持有锁）
func (s *Session) compressL0Locked() {
	if len(s.L1Pages) <= s.MaxL1Pages {
		return
	}

	// TODO: n由L1压缩百分比决定，该百分比将由配置文件决定
	// 取出最旧的 n 个 L1 pages 进行压缩到 L0
	n := len(s.L1Pages) - s.MaxL1Pages
	oldL1Pages := s.L1Pages[len(s.L1Pages)-n:]

	// 合并内容
	var mergedContent string
	if s.L0Page != nil {
		mergedContent = s.L0Page.Content + "\n\n"
	}
	for _, p := range oldL1Pages {
		mergedContent += p.Content + "\n\n"
	}

	// 创建新的 L0 page
	newL0Page := NewPage(PageTypeL0, s.L0Strategy)
	newL0Page.Content = mergedContent

	// 归档旧的 L1 pages
	for _, p := range oldL1Pages {
		if p.ArchivedFile != "" {
			// 追加到 L0 的归档文件
			// TODO: 实现归档文件合并
		}
	}

	// 更新 L0 page 和 L1 pages
	s.L0Page = newL0Page
	s.L1Pages = s.L1Pages[:s.MaxL1Pages]

	s.logger.Info("L0 compressed", "merged_pages", n)
}

// GetStats 返回 Session 统计信息
func (s *Session) GetStats() SessionStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	l2TokenCount := 0
	for _, p := range s.L2Pages {
		l2TokenCount += p.GetTokenCount()
	}

	l1TokenCount := 0
	for _, p := range s.L1Pages {
		l1TokenCount += p.GetTokenCount()
	}

	l0TokenCount := 0
	if s.L0Page != nil {
		l0TokenCount = s.L0Page.GetTokenCount()
	}

	return SessionStats{
		TotalL0Tokens: l0TokenCount,
		TotalL1Tokens: l1TokenCount,
		TotalL2Tokens: l2TokenCount,
		L1PageCount:   len(s.L1Pages),
		L2PageCount:   len(s.L2Pages),
	}
}

// SessionStats Session 统计信息
type SessionStats struct {
	TotalL0Tokens int `json:"total_l0_tokens"`
	TotalL1Tokens int `json:"total_l1_tokens"`
	TotalL2Tokens int `json:"total_l2_tokens"`
	L1PageCount   int `json:"l1_page_count"`
	L2PageCount   int `json:"l2_page_count"`
}

// Close 关闭 Session
func (s *Session) Close() {
	if s.compressWkr != nil {
		s.compressWkr.Stop()
	}
	s.logger.Info("Session closed", "id", s.ID)
}
