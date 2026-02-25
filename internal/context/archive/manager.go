package archive

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yourname/oxencode/internal/message"
	"github.com/yourname/oxencode/pkg/logger"
)

// Manager 归档管理器
type Manager struct {
	archiveDir string
	logger     logger.Logger
}

// ArchiveEntry 归档条目元数据
type ArchiveEntry struct {
	PageID    string    `json:"page_id"`
	PageType  string    `json:"page_type"`
	CreatedAt time.Time `json:"created_at"`
	FilePath  string    `json:"file_path"`
	TokenCount int      `json:"token_count"`
}

// NewManager 创建归档管理器
func NewManager(archiveDir string) (*Manager, error) {
	log := logger.New("context/archive")

	// 使用默认目录或配置目录
	if archiveDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		archiveDir = filepath.Join(homeDir, ".local", "share", "oxencode", "archive")
	}

	// 创建目录
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create archive directory: %w", err)
	}

	log.Info("Archive manager created", "dir", archiveDir)

	return &Manager{
		archiveDir: archiveDir,
		logger:     log,
	}, nil
}

// Archive 归档消息到文件系统
func (m *Manager) Archive(pageID string, pageType string, messages []message.Message) (string, error) {
	// 创建按日期分层的目录结构
	// 格式：archive/2025/02/25/{pageID}.json
	now := time.Now()
	dateDir := filepath.Join(
		m.archiveDir,
		fmt.Sprintf("%d", now.Year()),
		fmt.Sprintf("%02d", now.Month()),
		fmt.Sprintf("%02d", now.Day()),
	)

	if err := os.MkdirAll(dateDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create date directory: %w", err)
	}

	// 生成文件路径
	filePath := filepath.Join(dateDir, fmt.Sprintf("%s.json", pageID))

	// 创建归档数据
	archiveData := ArchiveData{
		PageID:    pageID,
		PageType:  pageType,
		CreatedAt: now,
		Messages:  messages,
	}

	// 序列化
	data, err := json.MarshalIndent(archiveData, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal messages: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write archive file: %w", err)
	}

	m.logger.Debug("Message archived", "page_id", pageID, "file", filePath)
	return filePath, nil
}

// ArchiveData 归档数据结构
type ArchiveData struct {
	PageID    string            `json:"page_id"`
	PageType  string            `json:"page_type"`
	CreatedAt time.Time         `json:"created_at"`
	Messages  []message.Message `json:"messages"`
}

// Read 从归档文件读取消息
func (m *Manager) Read(pageID string) ([]message.Message, error) {
	// 查找文件
	filePath, err := m.findFile(pageID)
	if err != nil {
		return nil, err
	}

	// 读取文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read archive file: %w", err)
	}

	// 反序列化
	var archiveData ArchiveData
	if err := json.Unmarshal(data, &archiveData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal messages: %w", err)
	}

	m.logger.Debug("Archive read", "page_id", pageID, "message_count", len(archiveData.Messages))
	return archiveData.Messages, nil
}

// Search 搜索归档消息
func (m *Manager) Search(query string, limit int) ([]ArchiveEntry, error) {
	m.logger.Debug("Searching archive", "query", query, "limit", limit)

	var results []ArchiveEntry

	// 遍历归档目录
	err := filepath.Walk(m.archiveDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过错误文件
		}

		if info.IsDir() || !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}

		// 读取并检查是否匹配
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var archiveData ArchiveData
		if err := json.Unmarshal(data, &archiveData); err != nil {
			return nil
		}

		// 简单关键词匹配
		matched := false
		queryLower := strings.ToLower(query)

		// 检查 PageID
		if strings.Contains(strings.ToLower(archiveData.PageID), queryLower) {
			matched = true
		}

		// 检查消息内容
		if !matched {
			for _, msg := range archiveData.Messages {
				if strings.Contains(strings.ToLower(msg.Content), queryLower) {
					matched = true
					break
				}
			}
		}

		if matched {
			results = append(results, ArchiveEntry{
				PageID:    archiveData.PageID,
				PageType:  archiveData.PageType,
				CreatedAt: archiveData.CreatedAt,
				FilePath:  path,
				TokenCount: estimateTokenCount(archiveData.Messages),
			})
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk archive directory: %w", err)
	}

	// 按时间排序（最新的在前）
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	// 限制结果数量
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// List 列出所有归档条目
func (m *Manager) List(limit int) ([]ArchiveEntry, error) {
	m.logger.Debug("Listing archives", "limit", limit)

	var results []ArchiveEntry

	err := filepath.Walk(m.archiveDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() || !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}

		// 读取元数据
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var archiveData ArchiveData
		if err := json.Unmarshal(data, &archiveData); err != nil {
			return nil
		}

		results = append(results, ArchiveEntry{
			PageID:    archiveData.PageID,
			PageType:  archiveData.PageType,
			CreatedAt: archiveData.CreatedAt,
			FilePath:  path,
			TokenCount: estimateTokenCount(archiveData.Messages),
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk archive directory: %w", err)
	}

	// 按时间排序（最新的在前）
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	// 限制结果数量
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// findFile 查找指定 pageID 的文件
func (m *Manager) findFile(pageID string) (string, error) {
	var found string

	err := filepath.Walk(m.archiveDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() || !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}

		if strings.TrimSuffix(info.Name(), ".json") == pageID {
			found = path
			return filepath.SkipAll // 找到后停止
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	if found == "" {
		return "", fmt.Errorf("archive not found for page_id: %s", pageID)
	}

	return found, nil
}

// Delete 删除归档
func (m *Manager) Delete(pageID string) error {
	filePath, err := m.findFile(pageID)
	if err != nil {
		return err
	}

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to remove archive file: %w", err)
	}

	m.logger.Debug("Archive deleted", "page_id", pageID)
	return nil
}

// GetStats 返回归档统计
func (m *Manager) GetStats() ArchiveStats {
	stats := ArchiveStats{}

	err := filepath.Walk(m.archiveDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() || !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}

		stats.FileCount++
		stats.TotalSize += info.Size()

		return nil
	})

	if err != nil {
		m.logger.Warn("Failed to calculate stats", "error", err)
	}

	return stats
}

// ArchiveStats 归档统计
type ArchiveStats struct {
	FileCount  int   `json:"file_count"`
	TotalSize  int64 `json:"total_size"`
}

// estimateTokenCount 估算 token 数量
func estimateTokenCount(messages []message.Message) int {
	count := 0
	for _, msg := range messages {
		count += len(msg.Content) / 4 // 粗略估算
	}
	return count
}
