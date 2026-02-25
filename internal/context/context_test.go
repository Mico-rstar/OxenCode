package context

import (
	"context"
	"testing"
	"time"

	"github.com/yourname/oxencode/internal/message"
)

// TestPage 测试 Page 基本功能
func TestPage(t *testing.T) {
	t.Run("NewPage", func(t *testing.T) {
		page := NewPage(PageTypeL2, nil)
		if page.ID == "" {
			t.Error("Expected page ID to be set")
		}
		if page.Type != PageTypeL2 {
			t.Errorf("Expected page type L2, got %s", page.Type)
		}
		if page.Messages == nil {
			t.Error("Expected messages to be initialized")
		}
	})

	t.Run("AddMessage", func(t *testing.T) {
		page := NewL2Page()
		msg := message.NewMessage(message.RoleUser, "test content")
		page.AddMessage(msg)

		if len(page.Messages) != 1 {
			t.Errorf("Expected 1 message, got %d", len(page.Messages))
		}
	})

	t.Run("GetTokenCount", func(t *testing.T) {
		page := NewL2Page()
		page.AddMessage(message.NewMessage(message.RoleUser, "hello world"))

		count := page.GetTokenCount()
		if count == 0 {
			t.Error("Expected token count to be greater than 0")
		}
	})

	t.Run("IsCompressed", func(t *testing.T) {
		// L2 page without content is not compressed
		page := NewL2Page()
		if page.IsCompressed() {
			t.Error("Expected L2 page to not be compressed")
		}

		// L1 page with content is compressed
		l1Page := NewPage(PageTypeL1, &CompressionStrategy{Schema: "test"})
		l1Page.Content = "compressed content"
		if !l1Page.IsCompressed() {
			t.Error("Expected L1 page with content to be compressed")
		}
	})
}

// TestPageCompress 测试 Page 压缩功能
func TestPageCompress(t *testing.T) {
	t.Run("CompressWithNilStrategy", func(t *testing.T) {
		page := NewL2Page()
		page.AddMessage(message.NewMessage(message.RoleUser, "test"))

		ctx := context.Background()
		err := page.Compress(ctx, nil)
		if err != nil {
			t.Errorf("Expected no error with nil compressor, got %v", err)
		}
		if page.Content == "" {
			t.Error("Expected content to be set")
		}
	})

	t.Run("CompressWithMockCompressor", func(t *testing.T) {
		page := NewPage(PageTypeL1, &CompressionStrategy{
			Schema:  "test schema",
			Timeout: 5 * time.Second,
		})
		page.AddMessage(message.NewMessage(message.RoleUser, "test content"))

		mockCompressor := NewMockCompressor(50)
		ctx := context.Background()
		err := page.Compress(ctx, mockCompressor)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if page.Content == "" {
			t.Error("Expected content to be set after compression")
		}
	})
}

// TestSession 测试 Session 基本功能
func TestSession(t *testing.T) {
	t.Run("NewSession", func(t *testing.T) {
		config := DefaultSessionConfig()
		config.Compressor = NewMockCompressor(100)

		session, err := NewSession(config)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if session.ID == "" {
			t.Error("Expected session ID to be set")
		}
		if session.MaxL1Pages != 10 {
			t.Errorf("Expected max L1 pages to be 10, got %d", session.MaxL1Pages)
		}

		session.Close()
	})

	t.Run("AddMessage", func(t *testing.T) {
		config := DefaultSessionConfig()
		config.Compressor = NewMockCompressor(100)
		session, _ := NewSession(config)
		defer session.Close()

		msg := message.NewMessage(message.RoleUser, "test message")
		session.AddMessage(msg)

		if len(session.L2Pages) != 1 {
			t.Errorf("Expected 1 L2 page, got %d", len(session.L2Pages))
		}
	})

	t.Run("GetContext", func(t *testing.T) {
		config := DefaultSessionConfig()
		config.SystemPrompt = "You are a test assistant."
		config.Compressor = NewMockCompressor(100)
		session, _ := NewSession(config)
		defer session.Close()

		// Add some messages
		session.AddMessage(message.NewMessage(message.RoleUser, "Hello"))
		session.AddMessage(message.NewMessage(message.RoleAssistant, "Hi there"))

		context := session.GetContext()
		if len(context) < 2 {
			t.Errorf("Expected at least 2 messages in context, got %d", len(context))
		}
	})

	t.Run("GetStats", func(t *testing.T) {
		config := DefaultSessionConfig()
		config.Compressor = NewMockCompressor(100)
		session, _ := NewSession(config)
		defer session.Close()

		session.AddMessage(message.NewMessage(message.RoleUser, "test content for token count"))

		stats := session.GetStats()
		if stats.TotalL2Tokens == 0 {
			t.Error("Expected L2 token count to be greater than 0")
		}
	})
}

// TestSessionCommit 测试 Session Commit 功能
func TestSessionCommit(t *testing.T) {
	t.Run("Commit", func(t *testing.T) {
		config := DefaultSessionConfig()
		config.Compressor = NewMockCompressor(100)
		session, _ := NewSession(config)
		defer session.Close()

		// Add messages
		session.AddMessage(message.NewMessage(message.RoleUser, "test"))

		ctx := context.Background()
		err := session.Commit(ctx)
		if err != nil {
			t.Errorf("Expected no error on commit, got %v", err)
		}

		// After commit, should have a new empty L2 page for new messages
		// (Commit creates a new L2 page and queues the old one for compression to L1)
		if len(session.L2Pages) < 1 {
			t.Errorf("Expected at least 1 L2 page after commit, got %d", len(session.L2Pages))
		}
	})

	t.Run("CommitEmptyL2", func(t *testing.T) {
		config := DefaultSessionConfig()
		config.Compressor = NewMockCompressor(100)
		session, _ := NewSession(config)
		defer session.Close()

		ctx := context.Background()
		err := session.Commit(ctx)
		if err != nil {
			t.Errorf("Expected no error on empty commit, got %v", err)
		}
	})
}

// TestCompressionStrategies 测试压缩策略
func TestCompressionStrategies(t *testing.T) {
	l0, l1, l2 := DefaultCompressionStrategies()

	if l0.MaxCompressionRate != 0.3 {
		t.Errorf("Expected L0 max rate 0.3, got %f", l0.MaxCompressionRate)
	}
	if l1.MaxCompressionRate != 0.5 {
		t.Errorf("Expected L1 max rate 0.5, got %f", l1.MaxCompressionRate)
	}
	if l2.MaxCompressionRate != 1.0 {
		t.Errorf("Expected L2 max rate 1.0, got %f", l2.MaxCompressionRate)
	}
}

// TestMockCompressor 测试 MockCompressor
func TestMockCompressor(t *testing.T) {
	mock := NewMockCompressor(50)
	ctx := context.Background()

	strategy := &CompressionStrategy{
		Schema: "test",
	}

	// Short content should pass through
	result, err := mock.Compress(ctx, "short", strategy)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != "short" {
		t.Errorf("Expected 'short', got '%s'", result)
	}

	// Long content should be truncated
	longStr := ""
	for i := 0; i < 100; i++ {
		longStr += "a"
	}
	result, err = mock.Compress(ctx, longStr, strategy)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(result) > 100 {
		t.Errorf("Expected result to be truncated to ~100 chars, got %d", len(result))
	}
}
