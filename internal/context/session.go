package context

import (
	"context"
	"errors"
	"os"
	"strings"
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
	SystemPrompt string  `json:"system_prompt"` // 系统 Prompt
	L0Page       *Page   `json:"l0_page"`       // 全局唯一的 L0 Page
	L1Pages      []*Page `json:"l1_pages"`      // L1 Pages 列表（按时间倒序，最新的在前）
	L2Page       *Page   `json:"l2_page"`       // 当前活跃的 L2 Page（单一）

	// 阈值配置
	MaxContextTokens int        `json:"max_context_tokens"` // 总上下文硬上限
	Thresholds       Thresholds `json:"thresholds"`         // 各层级阈值（计算后的绝对值）

	// 状态管理
	State            SessionState  `json:"state"`             // 当前状态
	CompressingPages []string      `json:"compressing_pages"` // 正在压缩的 page IDs
	compressDone     chan struct{} `json:"-"`                 // 压缩完成信号

	// 配置引用
	cfg *config.Config `json:"-"`

	// 压缩策略
	L0Strategy *CompressionStrategy `json:"l0_strategy"`
	L1Strategy *CompressionStrategy `json:"l1_strategy"`
	L2Strategy *CompressionStrategy `json:"l2_strategy"`

	// 异步压缩管理（用于L0）
	compressor  Compressor      `json:"-"` // 压缩器
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

// NewSession 创建新的 Session
// systemPrompt 由调用者传入，cfg 和 compressor 是必需的依赖
func NewSession(systemPrompt string, cfg *config.Config, compressor Compressor) (*Session, error) {
	log := logger.New("context/session")

	if compressor == nil {
		panic("Compressor is required, cannot create Session without it")
	}

	if cfg == nil {
		cfg = config.Get()
	}

	// 从Config创建策略
	var l0Strategy, l1Strategy, l2Strategy *CompressionStrategy
	var maxContextTokens int
	if cfg != nil {
		l0Strategy = NewCompressionStrategy(PageTypeL0, cfg)
		l1Strategy = NewCompressionStrategy(PageTypeL1, cfg)
		l2Strategy = NewCompressionStrategy(PageTypeL2, cfg)
		maxContextTokens = cfg.MaxContextTokens
	} else {
		l0Strategy, l1Strategy, l2Strategy = DefaultCompressionStrategies()
		maxContextTokens = 200000 // 默认值
	}

	// 确定归档目录：优先使用配置，否则使用默认值
	archiveDir := cfg.ArchiveDir
	if archiveDir == "" {
		archiveDir = "~/.local/share/oxencode/archive"
	}

	session := &Session{
		ID:               time.Now().Format("20060102-150405"),
		SystemPrompt:     systemPrompt,
		L0Page:           nil, // 初始时没有 L0 page
		L1Pages:          make([]*Page, 0),
		L2Page:           nil, // 初始时没有 L2 page
		MaxContextTokens: maxContextTokens,
		Thresholds:       NewThresholds(cfg),
		State:            StateNormal,
		CompressingPages: make([]string, 0),
		compressDone:     make(chan struct{}, 1),
		cfg:              cfg,
		L0Strategy:       l0Strategy,
		L1Strategy:       l1Strategy,
		L2Strategy:       l2Strategy,
		compressor:       compressor,
		ArchiveDir:       archiveDir,
		logger:           log,
		initialized:      true,
	}

	// 创建并启动压缩工作器（用于L0）
	workerConfig := DefaultCompressWorkerConfig()
	session.compressWkr = NewCompressWorker(compressor, workerConfig)
	go session.processCompressResults()

	return session, nil
}

// AddAtom 添加原子消息序列到 L2
// 这是新的主要接口，保证 assistant + tool_results 的原子性
func (s *Session) AddAtom(atom *message.AtomSequence) error {
	s.mu.Lock()

	// 估算新原子序列的token数
	newTokens := atom.GetTokenCount()

	// 写屏障：检查是否会超过总上限
	for s.calculateTotalTokensLocked()+newTokens > s.MaxContextTokens {
		if s.isInCompressingLocked() {
			// 正在压缩，等待完成
			s.mu.Unlock()
			s.waitForCompressComplete()
			s.mu.Lock()
		} else {
			// 没有在压缩，需要主动触发压缩
			triggered := s.triggerCompressLocked()
			if !triggered {
				// 无法触发L0压缩（L1不足），先强制提交L2到L1
				if s.L2Page != nil && len(s.L2Page.Atoms) > 0 {
					s.forceCommitL2Locked()
				}
				// 再次尝试触发压缩
				triggered = s.triggerCompressLocked()
			}
			if triggered {
				s.mu.Unlock()
				s.waitForCompressComplete()
				s.mu.Lock()
			} else {
				// 无法压缩也无法提交（L2为空），打破循环允许消息添加
				s.logger.Warn("Cannot compress or commit, allowing atom despite exceeding limit")
				break
			}
		}
	}

	// 确保有当前的 L2 page
	if s.L2Page == nil {
		s.L2Page = NewL2Page()
	}

	// 添加原子序列到 L2 page
	s.L2Page.AddAtom(atom)

	// 检查是否触发 SoftMaxL2
	if s.L2Page.GetTokenCount() > s.Thresholds.SoftMaxL2 {
		go s.checkAndCommitL2()
	}

	// 检查 L1 是否需要压缩（主动管理）
	l1Tokens := s.getL1TokensLocked()
	if l1Tokens > s.Thresholds.SoftMaxL1 && !s.isInCompressingLocked() {
		s.logger.Info("AddAtom: L1 exceeds SoftMax, triggering L0 compression",
			"l1_tokens", l1Tokens,
			"threshold", s.Thresholds.SoftMaxL1)
		s.startL0CompressLocked()
	}

	s.mu.Unlock()
	return nil
}

// AddMessage 添加消息到 Session（添加到 L2 Page）
// 注意：此方法保留向后兼容，内部创建单消息原子
func (s *Session) AddMessage(msg message.Message) error {
	atom := message.NewAtomSequence()
	switch(msg.Role) {
	case message.RoleAssistant:
		atom.SetAssistant(msg)
	case message.RoleUser:
		atom.SetUserMessage(msg)
	case message.RoleTool:
		s.logger.Error("Tool message should be added via AddAtom, creating standalone atom")
		return errors.New("Tool message should be added via AddAtom, creating standalone atom")
	default:
		s.logger.Error("Unknown messages, ignored by AddMessage method")
		return errors.New("Unknown messages, ignored by AddMessage method")
	}
	return s.AddAtom(atom)
}

// isInCompressingLocked 检查是否处于压缩状态（需要持有锁）
func (s *Session) isInCompressingLocked() bool {
	return s.State == StateCompressing
}

// waitForCompressComplete 等待压缩完成
func (s *Session) waitForCompressComplete() {
	timeout := time.Duration(s.cfg.CompressTimeout) * time.Second
	select {
	case <-s.compressDone:
	case <-time.After(timeout):
		s.logger.Warn("Compress wait timeout", "timeout", timeout)
	}
}

// triggerCompressLocked 触发L0压缩（需要持有锁）
// 返回是否成功触发了压缩
func (s *Session) triggerCompressLocked() bool {
	if s.isInCompressingLocked() {
		return false
	}

	// 检查L1是否需要压缩
	l1Tokens := s.getL1TokensLocked()
	if l1Tokens > s.Thresholds.SoftMaxL1 {
		s.startL0CompressLocked()
		return true
	}
	return false
}

// forceCommitL2Locked 强制提交L2到L1（不检查阈值，用于紧急分页）
// 整体提交，不会破坏原子性
func (s *Session) forceCommitL2Locked() {
	if s.L2Page == nil || len(s.L2Page.Atoms) == 0 {
		return
	}

	// 创建L1 Page，整体转移 Atoms
	l1Page := NewPage(PageTypeL1, s.L1Strategy)
	l1Page.Atoms = s.L2Page.Atoms
	l1Page.Messages = l1Page.BuildMessages()
	l1Page.Preprocess()

	// 添加到L1列表
	s.L1Pages = append([]*Page{l1Page}, s.L1Pages...)

	// 创建新的空L2
	s.L2Page = NewL2Page()

	s.logger.Info("L2 force committed to L1", "atoms", len(l1Page.Atoms))
}

// getL1TokensLocked 获取L1的总token数（需要持有锁）
func (s *Session) getL1TokensLocked() int {
	total := 0
	for _, p := range s.L1Pages {
		total += p.GetTokenCount()
	}
	return total
}

// checkAndCommitL2 检查并提交一半L2到L1
// 注意：此方法是异步调用的，不应该阻塞等待压缩完成
func (s *Session) checkAndCommitL2() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.L2Page == nil || len(s.L2Page.Atoms) == 0 {
		return
	}

	// 如果 L1 正在压缩，不等待，直接提交一半 L2 到 L1
	// 压缩完成后只会移除正在压缩的那些 L1 pages，新添加的不受影响

	// 检查是否需要触发L1->L0压缩（如果已经在压缩中，则不重复触发）
	if !s.isInCompressingLocked() {
		l1Tokens := s.getL1TokensLocked()
		if l1Tokens > s.Thresholds.SoftMaxL1 {
			s.startL0CompressLocked()
		}
	}

	// 提交一半L2到L1
	s.commitHalfL2Locked()
}

// commitHalfL2Locked 提交一半L2到L1（需要持有锁）
// 关键修改：按 Atom 边界分割，不切断任何原子序列
func (s *Session) commitHalfL2Locked() {
	if s.L2Page == nil || len(s.L2Page.Atoms) < 2 {
		return
	}

	atoms := s.L2Page.Atoms
	mid := len(atoms) / 2

	// 按 Atom 边界分割，不会破坏原子性
	oldAtoms := atoms[:mid]
	newAtoms := atoms[mid:]

	// 旧原子创建L1 Page
	l1Page := NewPage(PageTypeL1, s.L1Strategy)
	l1Page.Atoms = oldAtoms
	l1Page.Messages = l1Page.BuildMessages()
	l1Page.Preprocess()
	s.L1Pages = append([]*Page{l1Page}, s.L1Pages...)

	// 新原子保留为L2
	newL2 := NewL2Page()
	newL2.Atoms = newAtoms
	s.L2Page = newL2

	s.logger.Info("L2 committed to L1", "old_atoms", mid, "new_atoms", len(newAtoms))
}

// startL0CompressLocked 开始L0压缩（异步）（需要持有锁）
func (s *Session) startL0CompressLocked() {
	if s.isInCompressingLocked() {
		return
	}

	// 计算需要压缩的L1数量（一半）
	halfCount := len(s.L1Pages) / 2
	if halfCount == 0 && len(s.L1Pages) > 0 {
		halfCount = 1
	}
	if halfCount == 0 {
		return
	}

	// 取出最旧的L1 pages（列表末尾）
	toCompress := s.L1Pages[len(s.L1Pages)-halfCount:]
	pageIDs := make([]string, len(toCompress))
	for i, p := range toCompress {
		pageIDs[i] = string(p.ID)
	}

	// 设置状态
	s.State = StateCompressing
	s.CompressingPages = pageIDs

	// 准备压缩内容
	var content string
	if s.L0Page != nil {
		content = s.L0Page.Content + "\n\n"
	}
	for _, p := range toCompress {
		content += p.Render() + "\n\n"
	}

	// 创建压缩用的Page
	compressPage := NewPage(PageTypeL0, s.L0Strategy)
	compressPage.Content = content

	// 提交异步压缩任务
	s.compressWkr.Submit(compressPage, 1)

	s.logger.Info("L0 compression started", "pages", halfCount)
}

// Commit 提交当前 L2 page，创建 L1 page 并预处理
// 用于一轮交互结束后的显式提交
// 整体提交，不会破坏原子性
func (s *Session) Commit(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.L2Page == nil || len(s.L2Page.Atoms) == 0 {
		return nil // 没有需要提交的
	}

	// 归档 L2 page
	if _, err := s.L2Page.Archive(s.ArchiveDir); err != nil {
		s.logger.Warn("Failed to archive L2 page", "error", err)
	}

	// 创建新的 L1 page，整体转移 Atoms
	l1Page := NewPage(PageTypeL1, s.L1Strategy)
	l1Page.Atoms = s.L2Page.Atoms
	l1Page.Messages = l1Page.BuildMessages()

	// L1预处理（截断，不调用LLM）
	l1Page.Preprocess()

	// 添加到 L1Pages（即使 L1 正在压缩也不阻塞）
	// 压缩完成后只会移除正在压缩的那些 L1 pages，新添加的不受影响
	s.L1Pages = append([]*Page{l1Page}, s.L1Pages...)

	// 检查是否需要触发L0压缩（如果已经在压缩中，则不重复触发）
	if !s.isInCompressingLocked() {
		l1Tokens := s.getL1TokensLocked()
		if l1Tokens > s.Thresholds.SoftMaxL1 {
			s.startL0CompressLocked()
		}
	}

	// 替换当前的 L2 page 为新的空 page
	s.L2Page = NewL2Page()

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

	// 2. 添加 L0 page（如果有且非空）
	if s.L0Page != nil && s.L0Page.Content != "" {
		messages = append(messages, message.NewMessage(message.RoleSystem, s.L0Page.Render()))
	}

	// 3. 添加 L1 pages（按时间正序：从最旧到最新）
	// L1Pages 是倒序存储（最新在前），需要反向遍历
	// 注意：保留原始消息结构，而不是渲染成文本，以保持 tool_calls 和 tool 结果的正确关联
	for i := len(s.L1Pages) - 1; i >= 0; i-- {
		p := s.L1Pages[i]
		// 使用 ProcessedMessages（如果已预处理），否则使用 BuildMessages
		msgs := p.ProcessedMessages
		if msgs == nil {
			msgs = p.BuildMessages()
		}
		messages = append(messages, msgs...)
	}

	// 4. 添加 L2 page（使用 BuildMessages 从 Atoms 构建）
	if s.L2Page != nil {
		messages = append(messages, s.L2Page.BuildMessages()...)
	}

	// 调试：写入上下文窗口到文件
	s.writeContextDebugFile(messages)

	return messages
}

// writeContextDebugFile 写入上下文窗口到调试文件
func (s *Session) writeContextDebugFile(messages []message.Message) {
	debugFile := "/tmp/oxencode_context_debug.txt"

	var sb strings.Builder
	for _, msg := range messages {
		sb.WriteString(msg.Content)
		sb.WriteString("\n")
	}

	if err := os.WriteFile(debugFile, []byte(sb.String()), 0644); err != nil {
		s.logger.Warn("Failed to write context debug file", "error", err)
	}
}

// processCompressResults 处理压缩结果
func (s *Session) processCompressResults() {
	for result := range s.compressWkr.Results() {
		s.mu.Lock()

		if result.Error != nil {
			s.logger.Error("Compression failed", "page_id", result.PageID, "error", result.Error)
			// 即使失败也要结束压缩状态
			s.endCompressLocked()
			s.mu.Unlock()
			continue
		}

		// 更新L0 page
		s.L0Page = NewPage(PageTypeL0, s.L0Strategy)
		s.L0Page.Content = result.Content

		// 移除已压缩的L1 pages
		compressedSet := make(map[string]bool)
		for _, id := range s.CompressingPages {
			compressedSet[id] = true
		}
		newL1Pages := make([]*Page, 0)
		for _, p := range s.L1Pages {
			if !compressedSet[string(p.ID)] {
				newL1Pages = append(newL1Pages, p)
			}
		}
		s.L1Pages = newL1Pages

		// 结束压缩状态
		s.endCompressLocked()

		s.logger.Info("L0 compression completed")
		s.mu.Unlock()

		// 通知等待的协程
		select {
		case s.compressDone <- struct{}{}:
		default:
		}
	}
}

// endCompressLocked 结束压缩状态（需要持有锁）
func (s *Session) endCompressLocked() {
	s.State = StateNormal
	s.CompressingPages = make([]string, 0)
}

// GetStats 返回 Session 统计信息
func (s *Session) GetStats() SessionStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	l2TokenCount := 0
	if s.L2Page != nil {
		l2TokenCount = s.L2Page.GetTokenCount()
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
		State:         s.State,
	}
}

// SessionStats Session 统计信息
type SessionStats struct {
	TotalL0Tokens int           `json:"total_l0_tokens"`
	TotalL1Tokens int           `json:"total_l1_tokens"`
	TotalL2Tokens int           `json:"total_l2_tokens"`
	L1PageCount   int           `json:"l1_page_count"`
	State         SessionState  `json:"state"`
}

// Close 关闭 Session
func (s *Session) Close() {
	if s.compressWkr != nil {
		s.compressWkr.Stop()
	}
	close(s.compressDone)
}

// GetTotalTokenCount 获取当前总 token 数
func (s *Session) GetTotalTokenCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.calculateTotalTokensLocked()
}

// calculateTotalTokensLocked 计算总 token 数（需要持有锁）
func (s *Session) calculateTotalTokensLocked() int {
	total := 0
	if s.L0Page != nil {
		total += s.L0Page.GetTokenCount()
	}
	for _, p := range s.L1Pages {
		total += p.GetTokenCount()
	}
	if s.L2Page != nil {
		total += s.L2Page.GetTokenCount()
	}
	return total
}

// ForceCommit 强制提交当前 L2（用于紧急分页）
// 整体提交，不会破坏原子性
func (s *Session) ForceCommit(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.L2Page == nil || len(s.L2Page.Atoms) == 0 {
		return nil
	}

	// 归档
	if _, err := s.L2Page.Archive(s.ArchiveDir); err != nil {
		s.logger.Warn("Failed to archive L2 page", "error", err)
	}

	// 创建 L1，整体转移 Atoms
	l1Page := NewPage(PageTypeL1, s.L1Strategy)
	l1Page.Atoms = s.L2Page.Atoms
	l1Page.Messages = l1Page.BuildMessages()
	l1Page.Preprocess()

	s.L1Pages = append([]*Page{l1Page}, s.L1Pages...)

	// 检查 L0 压缩（如果已经在压缩中，则不重复触发）
	if !s.isInCompressingLocked() {
		l1Tokens := s.getL1TokensLocked()
		if l1Tokens > s.Thresholds.SoftMaxL1 {
			s.startL0CompressLocked()
		}
	}

	// 新 L2
	s.L2Page = NewL2Page()

	return nil
}

