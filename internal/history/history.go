package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Session 对话会话
type Session struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Messages  []Message `json:"messages"`
}

// Message 会话消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Time    time.Time `json:"time"`
}

// Manager 历史记录管理器
type Manager struct {
	sessions []Session
	current  *Session
	dataDir  string
}

// NewManager 创建历史记录管理器
func NewManager() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	dataDir := filepath.Join(homeDir, ".local", "share", "oxencode")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	return &Manager{
		sessions: []Session{},
		dataDir:  dataDir,
	}, nil
}

// NewSession 创建新会话
func (m *Manager) NewSession() error {
	session := Session{
		ID:        generateID(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  []Message{},
	}

	m.sessions = append(m.sessions, session)
	m.current = &m.sessions[len(m.sessions)-1]

	return m.save()
}

// AddMessage 添加消息到当前会话
func (m *Manager) AddMessage(role, content string) error {
	if m.current == nil {
		if err := m.NewSession(); err != nil {
			return err
		}
	}

	msg := Message{
		Role:    role,
		Content: content,
		Time:    time.Now(),
	}

	m.current.Messages = append(m.current.Messages, msg)
	m.current.UpdatedAt = time.Now()

	return m.save()
}

// GetCurrentSession 获取当前会话
func (m *Manager) GetCurrentSession() *Session {
	return m.current
}

// save 保存会话到文件
func (m *Manager) save() error {
	if m.current == nil {
		return nil
	}

	filePath := filepath.Join(m.dataDir, m.current.ID+".json")
	data, err := json.MarshalIndent(m.current, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

// generateID 生成会话 ID
func generateID() string {
	return time.Now().Format("20060102-150405")
}
