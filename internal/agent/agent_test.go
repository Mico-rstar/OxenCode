package agent

import (
	"context"
	"testing"
	"time"

	"github.com/yourname/oxencode/pkg/config"
	"github.com/yourname/oxencode/internal/message"
)

// TestNewAgent 测试 Agent 创建
func TestNewAgent(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.Config
		wantErr bool
	}{
		{
			name: "valid config with anthropic",
			config: &config.Config{
				Provider:    config.ProviderAnthropic,
				Model:       "claude-3-5-sonnet-20241022",
				MaxTokens:   4096,
				Temperature: 0.7,
			},
			wantErr: true, // 需要 API key，如果没有会报错
		},
		{
			name: "valid config with openai",
			config: &config.Config{
				Provider:    config.ProviderOpenAI,
				Model:       "gpt-4",
				MaxTokens:   4096,
				Temperature: 0.7,
			},
			wantErr: true, // 需要 API key
		},
		{
			name: "unsupported provider",
			config: &config.Config{
				Provider:    "unknown",
				Model:       "test-model",
				MaxTokens:   4096,
				Temperature: 0.7,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, err := NewAgent(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewAgent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && agent == nil {
				t.Error("NewAgent() returned nil agent without error")
			}
		})
	}
}

// TestAgentHistoryManagement 测试历史管理
func TestAgentHistoryManagement(t *testing.T) {
	// 创建一个 mock agent 用于测试历史管理
	// 注意：这个测试需要真实的 API key，如果没有则跳过
	cfg := &config.Config{
		Provider:    config.ProviderQwen,
		Model:       "qwen3-max",
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	agent, err := NewAgent(cfg)
	if err != nil {
		t.Skipf("Skipping test: %v (requires API key)", err)
		return
	}

	// 测试初始历史应该有系统消息
	history := agent.GetHistory()
	if len(history) != 1 {
		t.Errorf("Expected 1 system message in history, got %d", len(history))
	}
	if history[0].Role != message.RoleSystem {
		t.Errorf("Expected first message to be system role, got %v", history[0].Role)
	}

	// 测试清空历史
	agent.ClearHistory()
	history = agent.GetHistory()
	if len(history) != 0 {
		t.Errorf("Expected empty history after ClearHistory, got %d messages", len(history))
	}

	// 测试设置系统提示
	newSystemPrompt := "You are a test assistant"
	agent.SetSystemPrompt(newSystemPrompt)
	history = agent.GetHistory()
	if len(history) != 1 {
		t.Errorf("Expected 1 message after SetSystemPrompt, got %d", len(history))
	}
	if history[0].Content != newSystemPrompt {
		t.Errorf("Expected system prompt '%s', got '%s'", newSystemPrompt, history[0].Content)
	}
}

// TestBuildMessages 测试消息构建
func TestBuildMessages(t *testing.T) {
	cfg := &config.Config{
		Provider:    config.ProviderQwen,
		Model:       "qwen3-max",
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	agent, err := NewAgent(cfg)
	if err != nil {
		t.Skipf("Skipping test: %v (requires API key)", err)
		return
	}

	// 清空历史以便测试
	agent.ClearHistory()

	// 添加测试消息
	testMessages := []message.Message{
		message.NewMessage(message.RoleSystem, "You are a helpful assistant"),
		message.NewMessage(message.RoleUser, "Hello, how are you?"),
		message.NewMessage(message.RoleAssistant, "I'm doing well, thank you!"),
		message.NewMessage(message.RoleUser, "What can you do?"),
	}

	for _, msg := range testMessages {
		agent.history = append(agent.history, msg)
	}

	// 构建消息
	fantasyMessages := agent.buildMessages()

	// 验证
	if len(fantasyMessages) != len(testMessages) {
		t.Errorf("Expected %d messages, got %d", len(testMessages), len(fantasyMessages))
	}

	// 验证角色映射
	expectedRoles := []string{"system", "user", "assistant", "user"}
	for i, fm := range fantasyMessages {
		if string(fm.Role) != expectedRoles[i] {
			t.Errorf("Message %d: expected role %s, got %s", i, expectedRoles[i], fm.Role)
		}
	}
}

// TestChat 测试单轮对话（需要真实 API）
func TestChat(t *testing.T) {
	cfg := &config.Config{
		Provider:    config.ProviderQwen,
		Model:       "qwen3-max",  // 使用标准的模型名称
		MaxTokens:   4096,
		Temperature: 0.7,
		APIKey: "sk-8440a58764c846c88183bfec2d94d279",
	}

	agent, err := NewAgent(cfg)
	if err != nil {
		t.Skipf("Skipping test: %v (requires API key)", err)
		return
	}

	agent.SetSystemPrompt("You are a useful assistant")

	// 调试：打印当前历史
	history := agent.GetHistory()
	t.Logf("History before chat (%d messages):", len(history))
	for i, msg := range history {
		t.Logf("  [%d] Role: %s, Content: %s", i, msg.Role, msg.Content)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := agent.Chat(ctx, "Say 'Hello World'")
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if response == "" {
		t.Error("Chat() returned empty response")
	}

	// 验证历史已更新（用户消息 + 助手回复）
	history = agent.GetHistory()
	if len(history) < 2 {
		t.Errorf("Expected at least 2 messages in history (user + assistant), got %d", len(history))
	}

	t.Logf("Response: %s", response)
}

// TestChatStream 测试流式对话（需要真实 API）
func TestChatStream(t *testing.T) {
	cfg := &config.Config{
		Provider:    config.ProviderQwen,
		Model:       "qwen3-max",
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	agent, err := NewAgent(cfg)
	if err != nil {
		t.Skipf("Skipping test: %v (requires API key)", err)
		return
	}

	// 设置系统提示（保留历史中的系统消息）
	agent.SetSystemPrompt("You are a helpful assistant. Keep responses brief.")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	streamCh, errCh := agent.ChatStream(ctx, "Count from 1 to 3")

	// 收集响应
	var fullResponse string
	timeout := time.After(35 * time.Second)

	for {
		select {
		case delta, ok := <-streamCh:
			if !ok {
				// 流关闭
				goto Done
			}
			fullResponse += delta
		case err := <-errCh:
			if err != nil {
				t.Fatalf("ChatStream() error = %v", err)
			}
			goto Done
		case <-timeout:
			t.Fatal("ChatStream() timed out")
		case <-ctx.Done():
			t.Fatal("ChatStream() context cancelled")
		}
	}

Done:
	if fullResponse == "" {
		t.Error("ChatStream() returned empty response")
	}

	t.Logf("Full response: %s", fullResponse)

	// 验证历史已更新
	history := agent.GetHistory()
	if len(history) < 2 {
		t.Errorf("Expected at least 2 messages in history, got %d", len(history))
	}
}

// TestChatStreamCancellation 测试流式对话取消
func TestChatStreamCancellation(t *testing.T) {
	cfg := &config.Config{
		Provider:    config.ProviderQwen,
		Model:       "qwen3-max",
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	agent, err := NewAgent(cfg)
	if err != nil {
		t.Skipf("Skipping test: %v (requires API key)", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	streamCh, errCh := agent.ChatStream(ctx, "Tell me a long story")

	// 接收一些数据后取消
	receivedSome := false
	for {
		select {
		case delta, ok := <-streamCh:
			if !ok {
				goto Done
			}
			if delta != "" {
				receivedSome = true
				// 收到一些数据后取消
				cancel()
			}
		case err := <-errCh:
			if err != nil && err.Error() != "cancelled" {
				t.Logf("Expected cancellation or stream complete, got: %v", err)
			}
			goto Done
		case <-time.After(5 * time.Second):
			if !receivedSome {
				t.Error("Did not receive any data before timeout")
			}
			goto Done
		}
	}

Done:
	cancel() // 确保清理
}

// TestAgentWithEmptyInput 测试空输入处理
func TestAgentWithEmptyInput(t *testing.T) {
	cfg := &config.Config{
		Provider:    config.ProviderQwen,
		Model:       "qwen3-max",
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	agent, err := NewAgent(cfg)
	if err != nil {
		t.Skipf("Skipping test: %v (requires API key)", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 测试空输入
	streamCh, errCh := agent.ChatStream(ctx, "")

	select {
	case err := <-errCh:
		if err == nil {
			t.Error("Expected error for empty input, got nil")
		} else if err.Error() != "user message is empty" {
			t.Errorf("Expected 'user message is empty' error, got: %v", err)
		}
	case <-streamCh:
		t.Error("Should not receive stream data for empty input")
	case <-time.After(1 * time.Second):
		t.Fatal("Test timed out waiting for error")
	}
}

// TestSetSystemPrompt 测试设置系统提示
func TestSetSystemPrompt(t *testing.T) {
	cfg := &config.Config{
		Provider:    config.ProviderQwen,
		Model:       "qwen3-max",
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	agent, err := NewAgent(cfg)
	if err != nil {
		t.Skipf("Skipping test: %v (requires API key)", err)
		return
	}

	// 添加一些对话消息（初始已有系统消息）
	agent.history = append(agent.history,
		message.NewMessage(message.RoleUser, "Hello"),
		message.NewMessage(message.RoleAssistant, "Hi there!"),
	)

	// 设置新的系统提示
	newPrompt := "You are a coding expert"
	agent.SetSystemPrompt(newPrompt)

	history := agent.GetHistory()

	// 验证系统消息在开头
	if len(history) == 0 {
		t.Fatal("Expected non-empty history")
	}

	if history[0].Role != message.RoleSystem {
		t.Errorf("Expected first message to be system role, got %v", history[0].Role)
	}

	if history[0].Content != newPrompt {
		t.Errorf("Expected system prompt '%s', got '%s'", newPrompt, history[0].Content)
	}

	// 验证原有消息被保留（系统 + user + assistant）
	if len(history) < 3 {
		t.Errorf("Expected at least 3 messages (system + user + assistant), got %d", len(history))
	}
}

// BenchmarkChat 性能测试：单轮对话
func BenchmarkChat(b *testing.B) {
	cfg := &config.Config{
		Provider:    config.ProviderQwen,
		Model:       "qwen3-max",
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	agent, err := NewAgent(cfg)
	if err != nil {
		b.Skipf("Skipping benchmark: %v (requires API key)", err)
		return
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := agent.Chat(ctx, "Say 'OK'")
		if err != nil {
			b.Fatalf("Chat() error = %v", err)
		}
	}
}

// BenchmarkBuildMessages 性能测试：消息构建
func BenchmarkBuildMessages(b *testing.B) {
	cfg := &config.Config{
		Provider:    config.ProviderQwen,
		Model:       "qwen3-max",
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	agent, err := NewAgent(cfg)
	if err != nil {
		b.Skipf("Skipping benchmark: %v (requires API key)", err)
		return
	}

	// 添加一些测试消息
	for i := 0; i < 10; i++ {
		agent.history = append(agent.history,
			message.NewMessage(message.RoleUser, "Test message"),
			message.NewMessage(message.RoleAssistant, "Test response"),
		)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = agent.buildMessages()
	}
}

// TestAgentWithTools 测试 Agent 的工具集成
func TestAgentWithTools(t *testing.T) {
	// 使用 mock config 来避免需要 API key
	cfg := &config.Config{
		Provider:    config.ProviderQwen,
		Model:       "qwen3-max",
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	agent, err := NewAgent(cfg)
	if err != nil {
		t.Skipf("Skipping test: %v (requires API key)", err)
		return
	}

	t.Run("Agent has tool registry", func(t *testing.T) {
		if agent.toolRegistry == nil {
			t.Error("Agent should have tool registry")
		}
	})

	t.Run("Agent has environment", func(t *testing.T) {
		if agent.env == nil {
			t.Error("Agent should have environment")
		}
	})

	t.Run("P0 tools are registered", func(t *testing.T) {
		tools := agent.toolRegistry.Names()
		expectedTools := []string{"glob", "grep", "read"}

		for _, expected := range expectedTools {
			found := false
			for _, name := range tools {
				if name == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected tool %s not registered", expected)
			}
		}
	})

	t.Run("GetToolSchemas returns schemas", func(t *testing.T) {
		schemas := agent.GetToolSchemas()

		if len(schemas) == 0 {
			t.Error("Expected at least one tool schema")
		}

		// 验证每个 schema 有必要的字段
		for _, schema := range schemas {
			if _, ok := schema["name"]; !ok {
				t.Error("Tool schema should have 'name' field")
			}
			if _, ok := schema["description"]; !ok {
				t.Error("Tool schema should have 'description' field")
			}
			if _, ok := schema["input_schema"]; !ok {
				t.Error("Tool schema should have 'input_schema' field")
			}
		}
	})
}

// TestExecuteTool 测试工具执行
func TestExecuteTool(t *testing.T) {
	cfg := &config.Config{
		Provider:    config.ProviderQwen,
		Model:       "qwen3-max",
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	agent, err := NewAgent(cfg)
	if err != nil {
		t.Skipf("Skipping test: %v (requires API key)", err)
		return
	}

	ctx := context.Background()

	t.Run("Execute glob tool", func(t *testing.T) {
		result, err := agent.ExecuteTool(ctx, "glob", map[string]any{
			"pattern": "*.go",
		})

		if err != nil {
			t.Errorf("ExecuteTool failed: %v", err)
		}

		if result == "" {
			t.Error("Expected non-empty result")
		}

		t.Logf("Glob result: %s", result)
	})

	t.Run("Execute grep tool", func(t *testing.T) {
		result, err := agent.ExecuteTool(ctx, "grep", map[string]any{
			"pattern":      "package",
			"file_pattern": "*.go",
		})

		if err != nil {
			t.Errorf("ExecuteTool failed: %v", err)
		}

		if result == "" {
			t.Error("Expected non-empty result")
		}

		t.Logf("Grep result: %s", result)
	})

	t.Run("Execute read tool", func(t *testing.T) {
		result, err := agent.ExecuteTool(ctx, "read", map[string]any{
			"file_path": "agent_test.go",
			"limit":     10,
		})

		if err != nil {
			t.Errorf("ExecuteTool failed: %v", err)
		}

		if result == "" {
			t.Error("Expected non-empty result")
		}

		t.Logf("Read result (first 100 chars): %s...", result[:min(100, len(result))])
	})

	t.Run("Execute non-existent tool", func(t *testing.T) {
		_, err := agent.ExecuteTool(ctx, "nonexistent", map[string]any{})

		if err == nil {
			t.Error("Expected error for non-existent tool")
		}

		if err.Error() != "tool not found: nonexistent" {
			t.Errorf("Unexpected error message: %v", err)
		}
	})

	t.Run("Execute tool with invalid parameters", func(t *testing.T) {
		// read tool requires file_path
		_, err := agent.ExecuteTool(ctx, "read", map[string]any{})

		if err == nil {
			t.Error("Expected error for missing parameters")
		}
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
