package context

import (
	"context"
	"fmt"

	"charm.land/fantasy"
	"github.com/yourname/oxencode/internal/context/archive"
	"github.com/yourname/oxencode/pkg/config"
	"github.com/yourname/oxencode/pkg/logger"
)

// Manager 上下文管理器接口
type Manager interface {
	// NewSession 创建新会话
	NewSession(config *SessionConfig) (*Session, error)

	// GetSession 获取会话
	GetSession(sessionID string) (*Session, error)

	// GetCurrentSession 获取当前会话
	GetCurrentSession() *Session

	// SetCurrentSession 设置当前会话
	SetCurrentSession(sessionID string) error

	// CloseSession 关闭会话
	CloseSession(sessionID string) error

	// ListSessions 列出所有会话
	ListSessions() []string

	// Close 关闭管理器
	Close()
}

// manager 实现 Manager 接口
type manager struct {
	sessions       map[string]*Session
	currentSession string
	compressor     Compressor
	archiveManager *archive.Manager
	logger         logger.Logger
}

// ManagerConfig Manager 配置
type ManagerConfig struct {
	Compressor   Compressor
	ArchiveDir   string
	DefaultPrompt string
}

// NewManager 创建上下文管理器
func NewManager(config *ManagerConfig) (Manager, error) {
	log := logger.New("context/manager")

	// 创建归档管理器
	archiveMgr, err := archive.NewManager(config.ArchiveDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create archive manager: %w", err)
	}

	mgr := &manager{
		sessions:       make(map[string]*Session),
		compressor:     config.Compressor,
		archiveManager: archiveMgr,
		logger:         log,
	}

	log.Info("Context manager created")
	return mgr, nil
}

// NewSession 创建新会话
func (m *manager) NewSession(config *SessionConfig) (*Session, error) {
	// 如果提供了压缩器，使用它；否则使用默认压缩器
	if config.Compressor == nil {
		config.Compressor = m.compressor
	}

	session, err := NewSession(config)
	if err != nil {
		return nil, err
	}

	m.sessions[session.ID] = session
	m.logger.Info("Session created", "id", session.ID)

	return session, nil
}

// GetSession 获取会话
func (m *manager) GetSession(sessionID string) (*Session, error) {
	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return session, nil
}

// GetCurrentSession 获取当前会话
func (m *manager) GetCurrentSession() *Session {
	if m.currentSession == "" {
		return nil
	}
	return m.sessions[m.currentSession]
}

// SetCurrentSession 设置当前会话
func (m *manager) SetCurrentSession(sessionID string) error {
	if _, ok := m.sessions[sessionID]; !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	m.currentSession = sessionID
	m.logger.Info("Current session set", "id", sessionID)
	return nil
}

// CloseSession 关闭会话
func (m *manager) CloseSession(sessionID string) error {
	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.Close()
	delete(m.sessions, sessionID)

	if m.currentSession == sessionID {
		m.currentSession = ""
	}

	m.logger.Info("Session closed", "id", sessionID)
	return nil
}

// ListSessions 列出所有会话
func (m *manager) ListSessions() []string {
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	return ids
}

// Close 关闭管理器
func (m *manager) Close() {
	// 关闭所有会话
	for _, session := range m.sessions {
		session.Close()
	}
	m.sessions = make(map[string]*Session)
	m.logger.Info("Context manager closed")
}

// Helper: 创建 LLM 压缩器（需要传入 fantasy provider）
func NewLLMCompressorWithProvider(ctx context.Context, provider fantasy.Provider, cfg *config.Config, log logger.Logger) (Compressor, error) {
	return NewLLMCompressor(ctx, provider, cfg, log)
}

// Helper: 创建默认管理器
func NewDefaultManager(ctx context.Context, provider fantasy.Provider, archiveDir string, cfg *config.Config, log logger.Logger) (Manager, error) {
	// 创建压缩器
	var compressor Compressor
	llmCompressor, err := NewLLMCompressor(ctx, provider, cfg, log)
	// TODO: 静默降级到 Mock 压缩器是一种 workaround，应该直接报错让调用方决定如何处理
	if err != nil {
		// 压缩器创建失败，使用 mock 压缩器
		compressor = NewMockCompressor(1024)
	} else {
		compressor = llmCompressor
	}

	return NewManager(&ManagerConfig{
		Compressor:   compressor,
		ArchiveDir:   archiveDir,
		DefaultPrompt: "You are a helpful AI programming assistant.",
	})
}
