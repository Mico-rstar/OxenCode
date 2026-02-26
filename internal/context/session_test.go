package context

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/yourname/oxencode/internal/message"
)

// MockCompressorWithDelay 用于测试异步压缩行为的模拟压缩器
type MockCompressorWithDelay struct {
	MaxOutputLength int
	Delay           time.Duration
	CompressFunc    func(ctx context.Context, raw string, strategy *CompressionStrategy) (string, error)
}

func NewMockCompressorWithDelay(maxOutputLength int, delay time.Duration) *MockCompressorWithDelay {
	return &MockCompressorWithDelay{
		MaxOutputLength: maxOutputLength,
		Delay:           delay,
	}
}

func (c *MockCompressorWithDelay) Compress(ctx context.Context, raw string, strategy *CompressionStrategy) (string, error) {
	if c.Delay > 0 {
		time.Sleep(c.Delay)
	}
	if c.CompressFunc != nil {
		return c.CompressFunc(ctx, raw, strategy)
	}
	// 简单截断作为模拟压缩
	if len(raw) <= c.MaxOutputLength {
		return raw, nil
	}
	return "[Compressed] " + raw[:c.MaxOutputLength], nil
}

// TestNewSession 测试 Session 创建
func TestNewSession(t *testing.T) {
	t.Run("success with default config", func(t *testing.T) {
		config := DefaultSessionConfig()
		config.Compressor = NewMockCompressor(1000)

		session, err := NewSession(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer session.Close()

		if session.ID == "" {
			t.Error("expected session ID to be set")
		}
		if session.SystemPrompt != config.SystemPrompt {
			t.Errorf("expected system prompt %q, got %q", config.SystemPrompt, session.SystemPrompt)
		}
		if session.MaxL1Pages != config.MaxL1Pages {
			t.Errorf("expected max L1 pages %d, got %d", config.MaxL1Pages, session.MaxL1Pages)
		}
		if session.L0Page != nil {
			t.Error("expected L0 page to be nil initially")
		}
		if len(session.L1Pages) != 0 {
			t.Errorf("expected 0 L1 pages, got %d", len(session.L1Pages))
		}
		if len(session.L2Pages) != 0 {
			t.Errorf("expected 0 L2 pages, got %d", len(session.L2Pages))
		}
	})

	t.Run("success with custom config", func(t *testing.T) {
		config := &SessionConfig{
			SystemPrompt: "Custom system prompt",
			MaxL1Pages:   5,
			ArchiveDir:   "/tmp/test-archive",
			Compressor:   NewMockCompressor(500),
		}

		session, err := NewSession(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer session.Close()

		if session.SystemPrompt != "Custom system prompt" {
			t.Errorf("expected custom system prompt, got %q", session.SystemPrompt)
		}
		if session.MaxL1Pages != 5 {
			t.Errorf("expected max L1 pages 5, got %d", session.MaxL1Pages)
		}
	})

	t.Run("panic when compressor is nil", func(t *testing.T) {
		config := DefaultSessionConfig()
		config.Compressor = nil

		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic when compressor is nil")
			}
		}()

		_, _ = NewSession(config)
	})
}

// TestSession_AddMessage 测试添加消息到 Session
func TestSession_AddMessage(t *testing.T) {
	config := DefaultSessionConfig()
	config.Compressor = NewMockCompressor(1000)

	session, err := NewSession(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer session.Close()

	t.Run("add single message creates L2 page", func(t *testing.T) {
		msg := message.NewMessage(message.RoleUser, "Hello")
		session.AddMessage(msg)

		if len(session.L2Pages) != 1 {
			t.Errorf("expected 1 L2 page, got %d", len(session.L2Pages))
		}
		if len(session.L2Pages[0].Messages) != 1 {
			t.Errorf("expected 1 message in L2 page, got %d", len(session.L2Pages[0].Messages))
		}
	})

	t.Run("add multiple messages to same L2 page", func(t *testing.T) {
		session.AddMessage(message.NewMessage(message.RoleAssistant, "Hi there"))
		session.AddMessage(message.NewMessage(message.RoleUser, "How are you?"))

		if len(session.L2Pages) != 1 {
			t.Errorf("expected 1 L2 page, got %d", len(session.L2Pages))
		}
		if len(session.L2Pages[0].Messages) != 3 {
			t.Errorf("expected 3 messages in L2 page, got %d", len(session.L2Pages[0].Messages))
		}
	})
}

// TestSession_Commit 测试提交 L2 page 进行压缩
func TestSession_Commit(t *testing.T) {
	t.Run("commit creates L1 page and new L2 page", func(t *testing.T) {
		config := DefaultSessionConfig()
		config.Compressor = NewMockCompressor(1000)

		session, err := NewSession(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer session.Close()

		// Add messages
		session.AddMessage(message.NewMessage(message.RoleUser, "Test message"))

		// Should have 1 L2 page before commit
		if len(session.L2Pages) != 1 {
			t.Errorf("expected 1 L2 page before commit, got %d", len(session.L2Pages))
		}

		ctx := context.Background()
		err = session.Commit(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should have 1 L1 page and 2 L2 pages (new empty one at [0], old one at [1])
		if len(session.L1Pages) != 1 {
			t.Errorf("expected 1 L1 page, got %d", len(session.L1Pages))
		}
		if len(session.L2Pages) != 2 {
			t.Errorf("expected 2 L2 pages (new + old), got %d", len(session.L2Pages))
		}
		if len(session.L2Pages[0].Messages) != 0 {
			t.Errorf("expected new L2 page at [0] to be empty, got %d messages", len(session.L2Pages[0].Messages))
		}
		if len(session.L2Pages[1].Messages) != 1 {
			t.Errorf("expected old L2 page at [1] to have 1 message, got %d messages", len(session.L2Pages[1].Messages))
		}
	})

	t.Run("commit with no messages does nothing", func(t *testing.T) {
		config := DefaultSessionConfig()
		config.Compressor = NewMockCompressor(1000)

		session, err := NewSession(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer session.Close()

		ctx := context.Background()
		err = session.Commit(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(session.L1Pages) != 0 {
			t.Errorf("expected 0 L1 pages, got %d", len(session.L1Pages))
		}
		if len(session.L2Pages) != 0 {
			t.Errorf("expected 0 L2 pages, got %d", len(session.L2Pages))
		}
	})

	t.Run("multiple commits accumulate L1 pages", func(t *testing.T) {
		config := DefaultSessionConfig()
		config.Compressor = NewMockCompressor(1000)

		session, err := NewSession(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer session.Close()

		ctx := context.Background()
		for i := 0; i < 3; i++ {
			session.AddMessage(message.NewMessage(message.RoleUser, fmt.Sprintf("Message %d", i)))
			if err := session.Commit(ctx); err != nil {
				t.Fatalf("commit %d failed: %v", i, err)
			}
		}

		if len(session.L1Pages) != 3 {
			t.Errorf("expected 3 L1 pages, got %d", len(session.L1Pages))
		}
	})
}

// TestSession_GetContext 测试构建上下文窗口
func TestSession_GetContext(t *testing.T) {
	config := DefaultSessionConfig()
	config.SystemPrompt = "Test system prompt"
	config.Compressor = NewMockCompressor(1000)

	session, err := NewSession(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer session.Close()

	t.Run("empty session returns only system prompt", func(t *testing.T) {
		ctx := session.GetContext()

		if len(ctx) != 1 {
			t.Errorf("expected 1 message (system prompt), got %d", len(ctx))
		}
		if ctx[0].Role != message.RoleSystem {
			t.Errorf("expected system role, got %v", ctx[0].Role)
		}
		if ctx[0].Content != "Test system prompt" {
			t.Errorf("expected system prompt content, got %q", ctx[0].Content)
		}
	})

	t.Run("with messages returns all content", func(t *testing.T) {
		session.AddMessage(message.NewMessage(message.RoleUser, "User message"))
		session.AddMessage(message.NewMessage(message.RoleAssistant, "Assistant response"))

		ctx := session.GetContext()

		// System prompt + 2 messages
		if len(ctx) != 3 {
			t.Errorf("expected 3 messages, got %d", len(ctx))
		}
		if ctx[1].Role != message.RoleUser || ctx[1].Content != "User message" {
			t.Error("expected user message")
		}
		if ctx[2].Role != message.RoleAssistant || ctx[2].Content != "Assistant response" {
			t.Error("expected assistant message")
		}
	})

	t.Run("with L1 pages returns compressed content", func(t *testing.T) {
		// Create session with L1 pages
		config2 := DefaultSessionConfig()
		config2.Compressor = NewMockCompressorWithDelay(100, time.Millisecond*10)

		session2, err := NewSession(config2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer session2.Close()

		session2.AddMessage(message.NewMessage(message.RoleUser, "Test"))
		if err := session2.Commit(context.Background()); err != nil {
			t.Fatalf("commit failed: %v", err)
		}

		// Wait for async compression to complete
		time.Sleep(time.Millisecond * 50)

		ctx := session2.GetContext()

		// Should have system + L1 (as assistant message)
		if len(ctx) < 2 {
			t.Errorf("expected at least 2 messages, got %d", len(ctx))
		}
	})
}

// TestSession_GetStats 测试统计信息
func TestSession_GetStats(t *testing.T) {
	config := DefaultSessionConfig()
	config.Compressor = NewMockCompressor(1000)

	session, err := NewSession(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer session.Close()

	t.Run("empty session stats", func(t *testing.T) {
		stats := session.GetStats()

		if stats.TotalL0Tokens != 0 {
			t.Errorf("expected 0 L0 tokens, got %d", stats.TotalL0Tokens)
		}
		if stats.TotalL1Tokens != 0 {
			t.Errorf("expected 0 L1 tokens, got %d", stats.TotalL1Tokens)
		}
		if stats.TotalL2Tokens != 0 {
			t.Errorf("expected 0 L2 tokens, got %d", stats.TotalL2Tokens)
		}
		if stats.L1PageCount != 0 {
			t.Errorf("expected 0 L1 pages, got %d", stats.L1PageCount)
		}
		if stats.L2PageCount != 0 {
			t.Errorf("expected 0 L2 pages, got %d", stats.L2PageCount)
		}
	})

	t.Run("stats with messages", func(t *testing.T) {
		session.AddMessage(message.NewMessage(message.RoleUser, "Hello World"))
		session.AddMessage(message.NewMessage(message.RoleAssistant, "Hi there"))

		stats := session.GetStats()

		if stats.L2PageCount != 1 {
			t.Errorf("expected 1 L2 page, got %d", stats.L2PageCount)
		}
		if stats.TotalL2Tokens == 0 {
			t.Error("expected non-zero L2 tokens")
		}
	})
}

// TestSession_L0Compression 测试 L0 压缩触发
func TestSession_L0Compression(t *testing.T) {
	t.Run("L0 compression triggered when exceeding MaxL1Pages", func(t *testing.T) {
		config := DefaultSessionConfig()
		config.MaxL1Pages = 2
		// Use fast compressor
		config.Compressor = NewMockCompressorWithDelay(100, time.Millisecond*5)

		session, err := NewSession(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer session.Close()

		ctx := context.Background()

		// Add messages and commit 3 times (exceeds MaxL1Pages=2)
		for i := 0; i < 3; i++ {
			session.AddMessage(message.NewMessage(message.RoleUser, fmt.Sprintf("Message %d", i)))
			if err := session.Commit(ctx); err != nil {
				t.Fatalf("commit %d failed: %v", i, err)
			}
			// Wait for async compression
			time.Sleep(time.Millisecond * 20)
		}

		// Wait for all async operations to complete
		time.Sleep(time.Millisecond * 50)

		// Should have triggered L0 compression
		session.mu.RLock()
		hasL0 := session.L0Page != nil
		l1Count := len(session.L1Pages)
		session.mu.RUnlock()

		if !hasL0 {
			t.Error("expected L0 page to be created after exceeding MaxL1Pages")
		}
		if l1Count > config.MaxL1Pages {
			t.Errorf("expected L1 pages <= %d, got %d", config.MaxL1Pages, l1Count)
		}
	})
}

// TestSession_Close 测试关闭 Session
func TestSession_Close(t *testing.T) {
	config := DefaultSessionConfig()
	config.Compressor = NewMockCompressor(1000)

	session, err := NewSession(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Close should not panic
	session.Close()
}

// TestSession_Concurrency 测试并发安全性
func TestSession_Concurrency(t *testing.T) {
	config := DefaultSessionConfig()
	config.Compressor = NewMockCompressor(1000)

	session, err := NewSession(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer session.Close()

	done := make(chan bool)

	// Concurrent AddMessage calls
	go func() {
		for i := 0; i < 10; i++ {
			session.AddMessage(message.NewMessage(message.RoleUser, "Concurrent message"))
		}
		done <- true
	}()

	// Concurrent GetContext calls
	go func() {
		for i := 0; i < 10; i++ {
			_ = session.GetContext()
		}
		done <- true
	}()

	// Concurrent GetStats calls
	go func() {
		for i := 0; i < 10; i++ {
			_ = session.GetStats()
		}
		done <- true
	}()

	// Wait for all goroutines to complete
	<-done
	<-done
	<-done

	// Should not panic or deadlock
}
