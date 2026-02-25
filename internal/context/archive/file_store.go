package archive

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/yourname/oxencode/internal/message"
	"github.com/yourname/oxencode/pkg/logger"
)

// FileStore 基于文件的归档存储
// 提供比 Manager 更底层的文件操作
type FileStore struct {
	baseDir string
	cache   map[string][]message.Message
	mu      sync.RWMutex
	logger  logger.Logger
}

// NewFileStore 创建文件存储
func NewFileStore(baseDir string) (*FileStore, error) {
	log := logger.New("context/archive/filestore")

	if baseDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		baseDir = filepath.Join(homeDir, ".local", "share", "oxencode", "archive")
	}

	// 创建目录
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &FileStore{
		baseDir: baseDir,
		cache:   make(map[string][]message.Message),
		logger:  log,
	}, nil
}

// Store 存储消息到文件
func (fs *FileStore) Store(pageID string, messages []message.Message) (string, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// 生成文件路径
	filePath := fs.getFilePath(pageID)

	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// 序列化
	data, err := serializeMessages(messages)
	if err != nil {
		return "", fmt.Errorf("failed to serialize: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// 更新缓存
	fs.cache[pageID] = messages

	fs.logger.Debug("Stored messages", "page_id", pageID, "file", filePath, "count", len(messages))
	return filePath, nil
}

// Load 从文件加载消息
func (fs *FileStore) Load(pageID string) ([]message.Message, error) {
	fs.mu.RLock()
	// 先检查缓存
	if messages, ok := fs.cache[pageID]; ok {
		fs.mu.RUnlock()
		fs.logger.Debug("Cache hit", "page_id", pageID)
		return messages, nil
	}
	fs.mu.RUnlock()

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// 读取文件
	filePath := fs.getFilePath(pageID)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("archive not found for page_id: %s", pageID)
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// 反序列化
	messages, err := deserializeMessages(data)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize: %w", err)
	}

	// 更新缓存
	fs.cache[pageID] = messages

	fs.logger.Debug("Loaded messages", "page_id", pageID, "file", filePath, "count", len(messages))
	return messages, nil
}

// Exists 检查归档是否存在
func (fs *FileStore) Exists(pageID string) bool {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	if _, ok := fs.cache[pageID]; ok {
		return true
	}

	filePath := fs.getFilePath(pageID)
	_, err := os.Stat(filePath)
	return err == nil
}

// Delete 删除归档
func (fs *FileStore) Delete(pageID string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	filePath := fs.getFilePath(pageID)

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("archive not found: %s", pageID)
		}
		return fmt.Errorf("failed to remove file: %w", err)
	}

	// 清除缓存
	delete(fs.cache, pageID)

	fs.logger.Debug("Deleted archive", "page_id", pageID)
	return nil
}

// List 列出所有归档 ID
func (fs *FileStore) List() ([]string, error) {
	var ids []string

	err := filepath.Walk(fs.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() || !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}

		// 提取 pageID（去掉.json 后缀）
		pageID := strings.TrimSuffix(info.Name(), ".json")
		ids = append(ids, pageID)

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return ids, nil
}

// ClearCache 清除缓存
func (fs *FileStore) ClearCache() {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fs.cache = make(map[string][]message.Message)
	fs.logger.Debug("Cache cleared")
}

// GetCacheStats 返回缓存统计
func (fs *FileStore) GetCacheStats() CacheStats {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	totalMessages := 0
	for _, messages := range fs.cache {
		totalMessages += len(messages)
	}

	return CacheStats{
		CacheSize:     len(fs.cache),
		TotalMessages: totalMessages,
	}
}

// getFilePath 生成文件路径
func (fs *FileStore) getFilePath(pageID string) string {
	// 使用 pageID 的前两个字符作为子目录，避免单目录文件过多
	if len(pageID) >= 2 {
		return filepath.Join(fs.baseDir, pageID[:2], pageID+".json")
	}
	return filepath.Join(fs.baseDir, pageID+".json")
}

// serializeMessages 序列化消息
func serializeMessages(messages []message.Message) ([]byte, error) {
	return json.MarshalIndent(messages, "", "  ")
}

// deserializeMessages 反序列化消息
func deserializeMessages(data []byte) ([]message.Message, error) {
	var messages []message.Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

// CacheStats 缓存统计
type CacheStats struct {
	CacheSize     int `json:"cache_size"`
	TotalMessages int `json:"total_messages"`
}
