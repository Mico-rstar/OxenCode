package agent

import (
	"strings"
	"testing"

	"github.com/yourname/oxencode/pkg/config"
	"github.com/yourname/oxencode/pkg/logger"
)

// ========================================
// ToolExecutor Tests
// ========================================

func TestToolExecutor_TruncateOutput(t *testing.T) {
	tests := []struct {
		name            string
		output          string
		maxLength       int
		expectedLen     int
		expectTruncated bool
	}{
		{
			name:            "short output - no truncation",
			output:          "short text",
			maxLength:       100,
			expectTruncated: false,
		},
		{
			name:            "exact length - no truncation",
			output:          strings.Repeat("x", 100),
			maxLength:       100,
			expectTruncated: false,
		},
		{
			name:            "long output - truncation",
			output:          strings.Repeat("x", 1000),
			maxLength:       100,
			expectTruncated: true,
			expectedLen:     100 + 80, // maxLen + truncation message
		},
		{
			name:            "zero maxLength - use default",
			output:          strings.Repeat("x", 5000),
			maxLength:       0,
			expectTruncated: false, // default is 10000, so no truncation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				ReAct: config.ReActConfig{
					ToolOutputMaxLength: tt.maxLength,
				},
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
