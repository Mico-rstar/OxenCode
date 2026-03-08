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

		// Should have 1 L1 page and 1 L2 page (new empty one)
		// L2 内容被移动到 L1，L2 被替换为新的空 page
		if len(session.L1Pages) != 1 {
			t.Errorf("expected 1 L1 page, got %d", len(session.L1Pages))
		}
		if len(session.L2Pages) != 1 {
			t.Errorf("expected 1 L2 page, got %d", len(session.L2Pages))
		}
		if len(session.L2Pages[0].Messages) != 0 {
			t.Errorf("expected new L2 page to be empty, got %d messages", len(session.L2Pages[0].Messages))
		}
		// L1 page should contain the old L2 message
		if len(session.L1Pages[0].Messages) != 1 {
			t.Errorf("expected L1 page to have 1 message, got %d messages", len(session.L1Pages[0].Messages))
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

// TestSession_GetContext_FIFOOrder 测试消息按 FIFO 顺序返回
func TestSession_GetContext_FIFOOrder(t *testing.T) {
	config := DefaultSessionConfig()
	config.SystemPrompt = "Test system prompt"
	config.Compressor = NewMockCompressor(1000)
	config.MaxL1Pages = 5

	session, err := NewSession(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer session.Close()

	// 添加第一批消息并提交到 L1
	session.AddMessage(message.NewMessage(message.RoleUser, "L2-Msg1-User"))
	session.AddMessage(message.NewMessage(message.RoleAssistant, "L2-Msg1-Assistant"))
	if err := session.Commit(context.Background()); err != nil {
		t.Fatalf("commit 1 failed: %v", err)
	}

	// 添加第二批消息并提交到 L1
	session.AddMessage(message.NewMessage(message.RoleUser, "L2-Msg2-User"))
	session.AddMessage(message.NewMessage(message.RoleAssistant, "L2-Msg2-Assistant"))
	if err := session.Commit(context.Background()); err != nil {
		t.Fatalf("commit 2 failed: %v", err)
	}

	// 添加第三批消息（仍在 L2）
	session.AddMessage(message.NewMessage(message.RoleUser, "L2-Msg3-User"))
	session.AddMessage(message.NewMessage(message.RoleAssistant, "L2-Msg3-Assistant"))

	ctx := session.GetContext()

	// 预期顺序：System -> L1(旧) -> L1(新) -> L2(当前)
	// 验证消息数量
	if len(ctx) < 5 {
		t.Errorf("expected at least 5 messages, got %d", len(ctx))
		for i, msg := range ctx {
			t.Logf("ctx[%d]: role=%s, content=%q", i, msg.Role, msg.Content)
		}
	}

	// 验证系统消息
	if ctx[0].Role != message.RoleSystem {
		t.Errorf("expected system message at index 0, got %s", ctx[0].Role)
	}

	// 验证 L1 消息顺序（第一个 L1 应该在第二个 L1 之前）
	// 由于 L1 渲染内容，我们检查内容是否包含预期的消息
	l1Msgs := []string{}
	for _, msg := range ctx {
		if msg.Role == message.RoleAssistant && len(msg.Content) > 0 {
			// 检查是否是 L1 page 的内容（包含 "L2-Msg" 的消息）
			if containsAny(msg.Content, []string{"L2-Msg1", "L2-Msg2"}) {
				l1Msgs = append(l1Msgs, msg.Content)
			}
		}
	}

	// 验证 L1 消息按 FIFO 顺序（Msg1 应该在 Msg2 之前）
	if len(l1Msgs) >= 2 {
		idx1 := findIndex(l1Msgs[0], "L2-Msg1")
		idx2 := findIndex(l1Msgs[1], "L2-Msg2")
		if idx1 > idx2 {
			t.Error("L1 messages are not in FIFO order")
		}
	}

	// 验证 L2 消息顺序
	l2StartIdx := len(ctx) - 2 // 最后两条应该是 L2 消息
	if l2StartIdx >= 0 && l2StartIdx < len(ctx) {
		if ctx[l2StartIdx].Content != "L2-Msg3-User" {
			t.Errorf("expected L2-Msg3-User at index %d, got %q", l2StartIdx, ctx[l2StartIdx].Content)
		}
		if ctx[l2StartIdx+1].Content != "L2-Msg3-Assistant" {
			t.Errorf("expected L2-Msg3-Assistant at index %d, got %q", l2StartIdx+1, ctx[l2StartIdx+1].Content)
		}
	}

	t.Logf("Total messages: %d", len(ctx))
	for i, msg := range ctx {
		t.Logf("ctx[%d]: role=%s, content_preview=%q", i, msg.Role, truncate(msg.Content, 50))
	}
}

func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) && containsSubstring(s, substr) {
			return true
		}
	}
	return false
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func findIndex(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
