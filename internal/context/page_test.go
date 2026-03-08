package context

import (
	"strings"
	"testing"

	"github.com/yourname/oxencode/internal/message"
)

// TestPreprocess 测试预处理截断功能
func TestPreprocess(t *testing.T) {
	tests := []struct {
		name      string
		strategy  *CompressionStrategy
		messages  []message.Message
		checkFunc func(t *testing.T, processed []message.Message)
	}{
		{
			name: "truncate tool output",
			strategy: &CompressionStrategy{
				MaxToolOutputLength: 100,
				MaxAssistantLength:  0,
			},
			messages: []message.Message{
				{
					Role: message.RoleAssistant,
					ReActLoop: []message.ReActStep{
						{
							ToolCall: &message.ToolCall{
								Name:   "Read",
								Output: strings.Repeat("x", 200), // 200 chars, should be truncated to 100
							},
						},
					},
				},
			},
			checkFunc: func(t *testing.T, processed []message.Message) {
				output := processed[0].ReActLoop[0].ToolCall.Output
				expectedLen := 100 + len(TruncatedMarker)
				if len(output) != expectedLen {
					t.Errorf("expected %d chars, got %d chars", expectedLen, len(output))
				}
				if !strings.HasSuffix(output, TruncatedMarker) {
					t.Error("missing truncated marker")
				}
			},
		},
		{
			name: "truncate assistant message",
			strategy: &CompressionStrategy{
				MaxToolOutputLength: 0,
				MaxAssistantLength:  50,
			},
			messages: []message.Message{
				{
					Role:    message.RoleAssistant,
					Content: strings.Repeat("a", 100), // 100 chars, should be truncated to 50
				},
			},
			checkFunc: func(t *testing.T, processed []message.Message) {
				content := processed[0].Content
				expectedLen := 50 + len(TruncatedMarker)
				if len(content) != expectedLen {
					t.Errorf("expected %d chars, got %d chars", expectedLen, len(content))
				}
				if !strings.HasSuffix(content, TruncatedMarker) {
					t.Error("missing truncated marker")
				}
			},
		},
		{
			name: "user message not truncated",
			strategy: &CompressionStrategy{
				MaxToolOutputLength: 100,
				MaxAssistantLength:  50,
			},
			messages: []message.Message{
				{
					Role:    message.RoleUser,
					Content: strings.Repeat("u", 200), // User messages should NOT be truncated
				},
			},
			checkFunc: func(t *testing.T, processed []message.Message) {
				if len(processed[0].Content) != 200 {
					t.Errorf("user message should not be truncated, got %d chars", len(processed[0].Content))
				}
			},
		},
		{
			name: "no truncation when length is 0",
			strategy: &CompressionStrategy{
				MaxToolOutputLength: 0,
				MaxAssistantLength:  0,
			},
			messages: []message.Message{
				{
					Role:    message.RoleAssistant,
					Content: strings.Repeat("a", 100),
					ReActLoop: []message.ReActStep{
						{
							ToolCall: &message.ToolCall{
								Name:   "Read",
								Output: strings.Repeat("x", 200),
							},
						},
					},
				},
			},
			checkFunc: func(t *testing.T, processed []message.Message) {
				// Content should NOT be truncated
				if len(processed[0].Content) != 100 {
					t.Errorf("content should not be truncated when MaxAssistantLength is 0")
				}
				// Tool output should NOT be truncated
				if len(processed[0].ReActLoop[0].ToolCall.Output) != 200 {
					t.Errorf("tool output should not be truncated when MaxToolOutputLength is 0")
				}
			},
		},
		{
			name: "no truncation when content is short",
			strategy: &CompressionStrategy{
				MaxToolOutputLength: 1000,
				MaxAssistantLength:  1000,
			},
			messages: []message.Message{
				{
					Role:    message.RoleAssistant,
					Content: "short content",
					ReActLoop: []message.ReActStep{
						{
							ToolCall: &message.ToolCall{
								Name:   "Read",
								Output: "short output",
							},
						},
					},
				},
			},
			checkFunc: func(t *testing.T, processed []message.Message) {
				if processed[0].Content != "short content" {
					t.Error("short content should not be modified")
				}
				if processed[0].ReActLoop[0].ToolCall.Output != "short output" {
					t.Error("short tool output should not be modified")
				}
			},
		},
		{
			name: "truncate tool message",
			strategy: &CompressionStrategy{
				MaxToolOutputLength: 50,
				MaxAssistantLength:  0,
			},
			messages: []message.Message{
				{
					Role:    message.RoleTool,
					Content: strings.Repeat("t", 100), // Tool message content should be truncated
				},
			},
			checkFunc: func(t *testing.T, processed []message.Message) {
				content := processed[0].Content
				expectedLen := 50 + len(TruncatedMarker)
				if len(content) != expectedLen {
					t.Errorf("expected %d chars for tool message, got %d", expectedLen, len(content))
				}
			},
		},
		{
			name: "multiple tool calls",
			strategy: &CompressionStrategy{
				MaxToolOutputLength: 50,
				MaxAssistantLength:  0,
			},
			messages: []message.Message{
				{
					Role: message.RoleAssistant,
					ReActLoop: []message.ReActStep{
						{
							ToolCall: &message.ToolCall{
								Name:   "Read",
								Output: strings.Repeat("r", 100),
							},
						},
						{
							ToolCall: &message.ToolCall{
								Name:   "Grep",
								Output: strings.Repeat("g", 80),
							},
						},
					},
				},
			},
			checkFunc: func(t *testing.T, processed []message.Message) {
				expectedLen := 50 + len(TruncatedMarker)
				// Both tool outputs should be truncated
				if len(processed[0].ReActLoop[0].ToolCall.Output) != expectedLen {
					t.Errorf("first tool output not truncated correctly")
				}
				if len(processed[0].ReActLoop[1].ToolCall.Output) != expectedLen {
					t.Errorf("second tool output not truncated correctly")
				}
			},
		},
		{
			name:     "nil strategy",
			strategy: nil,
			messages: []message.Message{
				{
					Role:    message.RoleAssistant,
					Content: "test content",
				},
			},
			checkFunc: func(t *testing.T, processed []message.Message) {
				// When strategy is nil, Preprocess returns early without setting ProcessedMessages
				// So processed will be nil
				if processed != nil {
					t.Error("ProcessedMessages should be nil when strategy is nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := &Page{
				Type:     PageTypeL1,
				Strategy: tt.strategy,
				Messages: tt.messages,
			}
			page.Preprocess()
			tt.checkFunc(t, page.ProcessedMessages)
		})
	}
}

// TestRender 测试渲染输出功能
func TestRender(t *testing.T) {
	tests := []struct {
		name     string
		page     *Page
		contains []string
	}{
		{
			name: "L0 uses Content",
			page: &Page{
				Type:    PageTypeL0,
				Content: "compressed L0 content",
			},
			contains: []string{"compressed L0 content"},
		},
		{
			name: "L1 uses messages with role markers",
			page: &Page{
				Type: PageTypeL1,
				Messages: []message.Message{
					{Role: message.RoleUser, Content: "user input"},
					{Role: message.RoleAssistant, Content: "assistant response"},
				},
			},
			contains: []string{
				"[User]",
				"user input",
				"[Assistant]",
				"assistant response",
			},
		},
		{
			name: "L1 uses ProcessedMessages when available",
			page: &Page{
				Type: PageTypeL1,
				Messages: []message.Message{
					{Role: message.RoleUser, Content: "original user input"},
				},
				ProcessedMessages: []message.Message{
					{Role: message.RoleUser, Content: "processed user input"},
				},
			},
			contains: []string{"processed user input"},
		},
		{
			name: "L2 uses messages",
			page: &Page{
				Type: PageTypeL2,
				Messages: []message.Message{
					{Role: message.RoleUser, Content: "user message"},
				},
			},
			contains: []string{"[User]", "user message"},
		},
		{
			name: "tool call rendering",
			page: &Page{
				Type: PageTypeL1,
				Messages: []message.Message{
					{
						Role:    message.RoleAssistant,
						Content: "assistant content",
						ReActLoop: []message.ReActStep{
							{
								ToolCall: &message.ToolCall{
									Name:   "Read",
									Output: "file content here",
								},
							},
						},
					},
				},
			},
			contains: []string{
				"[Assistant]",
				"assistant content",
				"[Tool: Read]",
				"file content here",
			},
		},
		{
			name: "tool result rendering",
			page: &Page{
				Type: PageTypeL1,
				Messages: []message.Message{
					{
						Role:    message.RoleTool,
						Content: "tool result output",
					},
				},
			},
			contains: []string{
				"[Tool Result]",
				"tool result output",
			},
		},
		{
			name: "L0 returns Content even with Messages",
			page: &Page{
				Type:     PageTypeL0,
				Content:  "L0 compressed content",
				Messages: []message.Message{{Role: message.RoleUser, Content: "should not appear"}},
			},
			contains: []string{"L0 compressed content"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.page.Render()
			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("expected output to contain %q, got:\n%s", expected, result)
				}
			}
		})
	}
}

// TestRenderDoesNotContain tests that certain content is NOT in the output
func TestRenderDoesNotContain(t *testing.T) {
	t.Run("L0 ignores Messages when Content is set", func(t *testing.T) {
		page := &Page{
			Type:     PageTypeL0,
			Content:  "compressed",
			Messages: []message.Message{{Role: message.RoleUser, Content: "original message"}},
		}
		result := page.Render()
		if strings.Contains(result, "original message") {
			t.Error("L0 should not render Messages when Content is set")
		}
	})

	t.Run("L1 uses ProcessedMessages over Messages", func(t *testing.T) {
		page := &Page{
			Type: PageTypeL1,
			Messages: []message.Message{
				{Role: message.RoleUser, Content: "original"},
			},
			ProcessedMessages: []message.Message{
				{Role: message.RoleUser, Content: "processed"},
			},
		}
		result := page.Render()
		if strings.Contains(result, "original") {
			t.Error("L1 should use ProcessedMessages when available")
		}
	})
}