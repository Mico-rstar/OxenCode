package archive

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yourname/oxencode/internal/message"
)

// TestManager 测试归档管理器
func TestManager(t *testing.T) {
	// 创建临时目录作为测试归档目录
	tmpDir := t.TempDir()

	t.Run("NewManager", func(t *testing.T) {
		mgr, err := NewManager(tmpDir)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if mgr == nil {
			t.Fatal("Expected manager to be created")
		}
	})

	t.Run("Archive", func(t *testing.T) {
		mgr, _ := NewManager(tmpDir)

		messages := []message.Message{
			message.NewMessage(message.RoleUser, "test message 1"),
			message.NewMessage(message.RoleAssistant, "test response"),
		}

		filePath, err := mgr.Archive("test-page-1", "l1", messages)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if filePath == "" {
			t.Error("Expected file path to be returned")
		}

		// 检查文件是否存在
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Error("Expected archive file to exist")
		}
	})

	t.Run("Read", func(t *testing.T) {
		mgr, _ := NewManager(tmpDir)

		// 先归档
		messages := []message.Message{
			message.NewMessage(message.RoleUser, "test content"),
		}
		mgr.Archive("test-page-2", "l1", messages)

		// 读取
		readMessages, err := mgr.Read("test-page-2")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(readMessages) != 1 {
			t.Errorf("Expected 1 message, got %d", len(readMessages))
		}
	})

	t.Run("Delete", func(t *testing.T) {
		mgr, _ := NewManager(tmpDir)

		// 先归档
		messages := []message.Message{
			message.NewMessage(message.RoleUser, "to be deleted"),
		}
		mgr.Archive("test-page-del", "l1", messages)

		// 删除
		err := mgr.Delete("test-page-del")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// 验证删除
		_, err = mgr.Read("test-page-del")
		if err == nil {
			t.Error("Expected error when reading deleted archive")
		}
	})

	t.Run("GetStats", func(t *testing.T) {
		mgr, _ := NewManager(tmpDir)

		// 归档一些数据
		mgr.Archive("stats-1", "l1", []message.Message{
			message.NewMessage(message.RoleUser, "test"),
		})

		stats := mgr.GetStats()
		if stats.FileCount == 0 {
			t.Error("Expected file count to be greater than 0")
		}
		if stats.TotalSize == 0 {
			t.Error("Expected total size to be greater than 0")
		}
	})
}

// TestFileStore 测试文件存储
func TestFileStore(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("NewFileStore", func(t *testing.T) {
		store, err := NewFileStore(tmpDir)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if store == nil {
			t.Fatal("Expected store to be created")
		}
	})

	t.Run("StoreAndLoad", func(t *testing.T) {
		store, _ := NewFileStore(tmpDir)

		messages := []message.Message{
			message.NewMessage(message.RoleUser, "store test"),
			message.NewMessage(message.RoleAssistant, "response"),
		}

		// 存储
		path, err := store.Store("store-test-1", messages)
		if err != nil {
			t.Fatalf("Expected no error on store, got %v", err)
		}
		if path == "" {
			t.Error("Expected path to be returned")
		}

		// 加载
		loaded, err := store.Load("store-test-1")
		if err != nil {
			t.Fatalf("Expected no error on load, got %v", err)
		}
		if len(loaded) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(loaded))
		}
	})

	t.Run("Exists", func(t *testing.T) {
		store, _ := NewFileStore(tmpDir)

		// 存储一个
		store.Store("exists-test", []message.Message{
			message.NewMessage(message.RoleUser, "test"),
		})

		if !store.Exists("exists-test") {
			t.Error("Expected archive to exist")
		}
		if store.Exists("non-existent") {
			t.Error("Expected non-existent archive to not exist")
		}
	})

	t.Run("Cache", func(t *testing.T) {
		store, _ := NewFileStore(tmpDir)

		messages := []message.Message{
			message.NewMessage(message.RoleUser, "cache test"),
		}

		// 第一次存储
		store.Store("cache-test", messages)

		// 第一次加载（应该从文件）
		store.Load("cache-test")

		// 第二次加载（应该从缓存）
		stats := store.GetCacheStats()
		if stats.CacheSize == 0 {
			t.Error("Expected cache to have entries")
		}

		// 清除缓存
		store.ClearCache()
		stats = store.GetCacheStats()
		if stats.CacheSize != 0 {
			t.Error("Expected cache to be cleared")
		}
	})

	t.Run("List", func(t *testing.T) {
		// Use a separate temp directory for this test to avoid pollution
		listTmpDir := t.TempDir()
		store, _ := NewFileStore(listTmpDir)

		// 存储几个
		store.Store("list-1", []message.Message{message.NewMessage(message.RoleUser, "1")})
		store.Store("list-2", []message.Message{message.NewMessage(message.RoleUser, "2")})
		store.Store("list-3", []message.Message{message.NewMessage(message.RoleUser, "3")})

		ids, err := store.List()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(ids) != 3 {
			t.Errorf("Expected 3 IDs, got %d", len(ids))
		}
	})

	t.Run("Delete", func(t *testing.T) {
		store, _ := NewFileStore(tmpDir)

		store.Store("delete-test", []message.Message{
			message.NewMessage(message.RoleUser, "to delete"),
		})

		err := store.Delete("delete-test")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if store.Exists("delete-test") {
			t.Error("Expected archive to be deleted")
		}
	})
}

// TestEstimateTokenCount 测试 token 估算
func TestEstimateTokenCount(t *testing.T) {
	messages := []message.Message{
		message.NewMessage(message.RoleUser, "hello world"), // ~3 tokens
	}

	count := estimateTokenCount(messages)
	if count == 0 {
		t.Error("Expected token count to be greater than 0")
	}
}

// TestFileStorePathGeneration 测试文件路径生成
func TestFileStorePathGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewFileStore(tmpDir)

	// 测试长 ID 生成子目录
	longID := "abcdef123456"
	expectedSubdir := filepath.Join(tmpDir, "ab", longID+".json")
	actualPath := store.getFilePath(longID)

	// 检查是否包含正确的子目录
	if actualPath != expectedSubdir {
		t.Errorf("Expected path %s, got %s", expectedSubdir, actualPath)
	}

	// 测试短 ID 不生成子目录
	shortID := "a"
	expectedShort := filepath.Join(tmpDir, shortID+".json")
	actualShort := store.getFilePath(shortID)

	if actualShort != expectedShort {
		t.Errorf("Expected path %s, got %s", expectedShort, actualShort)
	}
}
