package archive

import (
	"os"
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

	t.Run("ReadNotFound", func(t *testing.T) {
		mgr, _ := NewManager(tmpDir)

		_, err := mgr.Read("non-existent-page")
		if err == nil {
			t.Error("Expected error when reading non-existent page")
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

	t.Run("List", func(t *testing.T) {
		// Use a separate temp directory for this test to avoid pollution
		listTmpDir := t.TempDir()
		mgr, _ := NewManager(listTmpDir)

		// 归档几个
		mgr.Archive("list-1", "l1", []message.Message{message.NewMessage(message.RoleUser, "1")})
		mgr.Archive("list-2", "l1", []message.Message{message.NewMessage(message.RoleUser, "2")})
		mgr.Archive("list-3", "l1", []message.Message{message.NewMessage(message.RoleUser, "3")})

		entries, err := mgr.List(10)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(entries) != 3 {
			t.Errorf("Expected 3 entries, got %d", len(entries))
		}
	})

	t.Run("Search", func(t *testing.T) {
		searchTmpDir := t.TempDir()
		mgr, _ := NewManager(searchTmpDir)

		// 归档带关键词的消息
		mgr.Archive("search-1", "l1", []message.Message{
			message.NewMessage(message.RoleUser, "hello world"),
		})
		mgr.Archive("search-2", "l1", []message.Message{
			message.NewMessage(message.RoleUser, "foo bar"),
		})

		// 搜索
		results, err := mgr.Search("hello", 10)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}
	})

	t.Run("EmptyArchive", func(t *testing.T) {
		mgr, _ := NewManager(tmpDir)

		messages := []message.Message{}
		filePath, err := mgr.Archive("empty-page", "l1", messages)
		if err != nil {
			t.Fatalf("Expected no error on empty archive, got %v", err)
		}
		if filePath == "" {
			t.Error("Expected file path to be returned for empty archive")
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
