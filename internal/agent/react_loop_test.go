package agent

import (
	"context"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/yourname/oxencode/internal/message"
	"github.com/yourname/oxencode/internal/tools"
	"github.com/yourname/oxencode/pkg/config"
	"github.com/yourname/oxencode/pkg/logger"
)

// ========================================
// ToolExecutor Tests
// ========================================

func TestToolExecutor_TruncateOutput(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		maxLength      int
		expectedLen    int
		expectTruncated bool
	}{
		{
			name:           "short output - no truncation",
			output:         "short text",
			maxLength:      100,
			expectTruncated: false,
		},
		{
			name:           "exact length - no truncation",
			output:         strings.Repeat("x", 100),
			maxLength:      100,
			expectTruncated: false,
		},
		{
			name:           "long output - truncation",
			output:         strings.Repeat("x", 1000),
			maxLength:      100,
			expectTruncated: true,
			expectedLen:    100 + 80, // maxLen + truncation message
		},
		{
			name:           "zero maxLength - use default",
			output:         strings.Repeat("x", 5000),
			maxLength:      0,
			expectTruncated: false, // default is 10000, so no truncation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				ToolOutputMaxLength: tt.maxLength,
			}

			executor := NewToolExecutor(nil, nil, cfg, logger.New("test"))

			result := executor.truncateOutput(tt.output, "test_tool")

			if tt.expectTruncated {
				if !strings.Contains(result, "truncated") {
					t.Errorf("expected truncated message in output")
				}
				if tt.expectedLen > 0 && len(result) > tt.expectedLen+50 {
					t.Errorf("output too long: got %d, expected ~%d", len(result), tt.expectedLen)
				}
			} else {
				if result != tt.output {
					t.Errorf("unexpected truncation: got len %d, want %d", len(result), len(tt.output))
				}
			}
		})
	}
}

func TestToolExecutor_GetToolOutputMaxLength(t *testing.T) {
	t.Run("with config value", func(t *testing.T) {
		cfg := &config.Config{ToolOutputMaxLength: 5000}
		executor := NewToolExecutor(nil, nil, cfg, logger.New("test"))

		if got := executor.getToolOutputMaxLength(); got != 5000 {
			t.Errorf("expected 5000, got %d", got)
		}
	})

	t.Run("without config - default", func(t *testing.T) {
		executor := NewToolExecutor(nil, nil, nil, logger.New("test"))

		if got := executor.getToolOutputMaxLength(); got != 10000 {
			t.Errorf("expected default 10000, got %d", got)
		}
	})
}



func TestMessageBuilder_ConvertMessage(t *testing.T) {
	builder := &MessageBuilder{
		session: nil,
		config:  nil,
		logger:  logger.New("test"),
	}

	t.Run("user message", func(t *testing.T) {
		msg := message.NewMessage(message.RoleUser, "Hello")
		result := builder.convertMessage(msg)

		if result.Role != fantasy.MessageRoleUser {
			t.Errorf("expected user role, got %s", result.Role)
		}
	})

	t.Run("assistant message", func(t *testing.T) {
		msg := message.NewMessage(message.RoleAssistant, "Hi there")
		result := builder.convertMessage(msg)

		if result.Role != fantasy.MessageRoleAssistant {
			t.Errorf("expected assistant role, got %s", result.Role)
		}
	})

	t.Run("system message", func(t *testing.T) {
		msg := message.NewMessage(message.RoleSystem, "System prompt")
		result := builder.convertMessage(msg)

		if result.Role != fantasy.MessageRoleSystem {
			t.Errorf("expected system role, got %s", result.Role)
		}
	})

	t.Run("tool message", func(t *testing.T) {
		msg := message.NewMessage(message.RoleTool, "tool output")
		result := builder.convertMessage(msg)

		// Tool messages are converted to user messages with prefix
		if result.Role != fantasy.MessageRoleUser {
			t.Errorf("expected user role for tool message, got %s", result.Role)
		}
	})
}

// ========================================
// ReActLoop Tests
// ========================================

func TestReActLoop_GetMaxIterations(t *testing.T) {
	t.Run("default value", func(t *testing.T) {
		reactLoop := &ReActLoop{
			cfg:    nil,
			logger: logger.New("test"),
		}

		if got := reactLoop.getMaxIterations(); got != 50 {
			t.Errorf("expected default 50, got %d", got)
		}
	})
}

func TestReActLoop_FormatToolInput(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    string
		expected string
	}{
		{
			name:     "empty input",
			toolName: "glob",
			input:    "",
			expected: "glob()",
		},
		{
			name:     "with input",
			toolName: "read",
			input:    `{"file": "test.go"}`,
			expected: `read({"file": "test.go"})`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatToolInput(tt.toolName, tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}



// ========================================
// Callback Tests
// ========================================

func TestReActLoop_SetCallbacks(t *testing.T) {
	reactLoop := &ReActLoop{
		callbacks: &Callbacks{},
		logger:    logger.New("test"),
	}

	callbacks := &Callbacks{
		OnThought: func(text string) {},
		OnAction:  func(toolName, input string) {},
	}

	reactLoop.SetCallbacks(callbacks)

	if reactLoop.callbacks.OnThought == nil {
		t.Error("OnThought callback not set")
	}
	if reactLoop.callbacks.OnAction == nil {
		t.Error("OnAction callback not set")
	}
}

func TestReActLoop_SetCallbacks_Nil(t *testing.T) {
	reactLoop := &ReActLoop{
		callbacks: &Callbacks{
			OnThought: func(text string) {},
		},
		logger: logger.New("test"),
	}

	// Setting nil should not clear existing callbacks
	reactLoop.SetCallbacks(nil)

	if reactLoop.callbacks.OnThought == nil {
		t.Error("OnThought callback should not be cleared")
	}
}

// ========================================
// ToolResult Tests
// ========================================

func TestToolResult(t *testing.T) {
	t.Run("success result", func(t *testing.T) {
		result := &ToolResult{
			Output:  "tool output",
			IsError: false,
		}

		if result.IsError {
			t.Error("should not be error")
		}
		if result.Output != "tool output" {
			t.Errorf("unexpected output: %s", result.Output)
		}
	})

	t.Run("error result", func(t *testing.T) {
		result := &ToolResult{
			Output:  "",
			Error:   "something went wrong",
			IsError: true,
		}

		if !result.IsError {
			t.Error("should be error")
		}
		if result.Error != "something went wrong" {
			t.Errorf("unexpected error: %s", result.Error)
		}
	})
}

// ========================================
// StreamEvent Tests
// ========================================

func TestStreamEvent(t *testing.T) {
	t.Run("content event", func(t *testing.T) {
		event := StreamEvent{
			Type:    "content",
			Content: "Hello",
		}

		if event.Type != "content" {
			t.Errorf("expected content type, got %s", event.Type)
		}
	})

	t.Run("action event with tool name", func(t *testing.T) {
		event := StreamEvent{
			Type:     "action",
			Content:  "glob(\"*.go\")",
			ToolName: "glob",
		}

		if event.ToolName != "glob" {
			t.Errorf("expected glob tool, got %s", event.ToolName)
		}
	})

	t.Run("error event", func(t *testing.T) {
		event := StreamEvent{
			Type:  "error",
			Error: context.Canceled,
		}

		if event.Type != "error" {
			t.Errorf("expected error type, got %s", event.Type)
		}
		if event.Error == nil {
			t.Error("expected error to be set")
		}
	})
}

// ========================================
// RunResult Tests
// ========================================

func TestRunResult(t *testing.T) {
	result := &RunResult{
		Content:      "Final response",
		FinishReason: "stop",
		Steps:        5,
	}

	if result.Content != "Final response" {
		t.Errorf("unexpected content: %s", result.Content)
	}
	if result.Steps != 5 {
		t.Errorf("expected 5 steps, got %d", result.Steps)
	}
}

// ========================================
// Integration-style Tests (require mocking)
// ========================================

// MockLanguageModel for testing ReActLoop without real LLM
type MockLanguageModel struct {
	GenerateFunc func(ctx context.Context, call fantasy.Call) (*fantasy.Response, error)
	StreamFunc   func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error)
}

func (m *MockLanguageModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	if m.GenerateFunc != nil {
		return m.GenerateFunc(ctx, call)
	}
	return &fantasy.Response{}, nil
}

func (m *MockLanguageModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	if m.StreamFunc != nil {
		return m.StreamFunc(ctx, call)
	}
	return func(yield func(fantasy.StreamPart) bool) {}, nil
}

func (m *MockLanguageModel) Provider() string { return "mock" }
func (m *MockLanguageModel) Model() string    { return "mock-model" }

// MockRegistry for testing without real tools
type MockRegistry struct {
	tools map[string]tools.Tool
}

func NewMockRegistry() *MockRegistry {
	return &MockRegistry{
		tools: make(map[string]tools.Tool),
	}
}

func (r *MockRegistry) Register(tool tools.Tool) {
	r.tools[tool.Name()] = tool
}

func (r *MockRegistry) Get(name string) tools.Tool {
	return r.tools[name]
}

func (r *MockRegistry) List() []tools.Tool {
	result := make([]tools.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

func (r *MockRegistry) Names() []string {
	result := make([]string, 0, len(r.tools))
	for name := range r.tools {
		result = append(result, name)
	}
	return result
}

func (r *MockRegistry) GetToolSchemas() []map[string]any {
	return nil
}