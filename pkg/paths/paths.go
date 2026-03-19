package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/yourname/oxencode/pkg/logger"
)

const (
	// AppName 应用名称
	AppName = "oxencode"
)

var (
	baseDir     string
	baseDirOnce sync.Once
	initErr     error
)

// Init 初始化路径模块，创建基础目录
func Init() error {
	baseDirOnce.Do(func() {
		log := logger.New("paths")

		homeDir, err := os.UserHomeDir()
		if err != nil {
			initErr = fmt.Errorf("failed to get home directory: %w", err)
			return
		}

		baseDir = filepath.Join(homeDir, "."+AppName)

		// 创建基础目录
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			initErr = fmt.Errorf("failed to create base directory: %w", err)
			return
		}

		log.Info("Paths initialized", "base_dir", baseDir)
	})

	return initErr
}

// BaseDir 返回应用基础目录 ~/.oxencode
func BaseDir() string {
	return baseDir
}

// ConfigFile 返回配置文件路径 ~/.oxencode/config.toml
func ConfigFile() string {
	return filepath.Join(baseDir, "config.toml")
}

// HistoryDir 返回历史记录目录 ~/.oxencode/history
func HistoryDir() string {
	return filepath.Join(baseDir, "history")
}

// ArchiveDir 返回归档目录 ~/.oxencode/archive
func ArchiveDir() string {
	return filepath.Join(baseDir, "archive")
}

// MemoryDir 返回记忆目录 ~/.oxencode/memory
func MemoryDir() string {
	return filepath.Join(baseDir, "memory")
}

// MetadataDB 返回元数据数据库路径 ~/.oxencode/metadata.db
func MetadataDB() string {
	return filepath.Join(baseDir, "metadata.db")
}

// ChromaDir 返回向量数据库目录 ~/.oxencode/chromadb
func ChromaDir() string {
	return filepath.Join(baseDir, "chromadb")
}

// EnsureDir 确保目录存在
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}