package memory

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFlexibleTime_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		jsonStr  string
		wantErr  bool
		expected string // Expected time in RFC3339 format
	}{
		{
			name:     "Python isoformat without timezone",
			jsonStr:  `{"created_at": "2026-03-20T19:26:05.277144"}`,
			wantErr:  false,
			expected: "2026-03-20T19:26:05Z", // Should be converted to UTC
		},
		{
			name:     "RFC3339 with timezone",
			jsonStr:  `{"created_at": "2026-03-20T19:26:05Z"}`,
			wantErr:  false,
			expected: "2026-03-20T19:26:05Z",
		},
		{
			name:     "RFC3339Nano with timezone",
			jsonStr:  `{"created_at": "2026-03-20T19:26:05.123456789Z"}`,
			wantErr:  false,
			expected: "2026-03-20T19:26:05Z",
		},
		{
			name:     "Without microseconds",
			jsonStr:  `{"created_at": "2026-03-20T19:26:05"}`,
			wantErr:  false,
			expected: "2026-03-20T19:26:05Z",
		},
		{
			name:     "Space separated",
			jsonStr:  `{"created_at": "2026-03-20 19:26:05"}`,
			wantErr:  false,
			expected: "2026-03-20T19:26:05Z",
		},
		{
			name:     "Date only",
			jsonStr:  `{"created_at": "2026-03-20"}`,
			wantErr:  false,
			expected: "2026-03-20T00:00:00Z",
		},
		{
			name:     "Empty string",
			jsonStr:  `{"created_at": ""}`,
			wantErr:  false,
			expected: "0001-01-01T00:00:00Z", // Empty time becomes zero value
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp struct {
				CreatedAt FlexibleTime `json:"created_at"`
			}
			err := json.Unmarshal([]byte(tt.jsonStr), &resp)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				got := resp.CreatedAt.Format(time.RFC3339)
				if got != tt.expected {
					t.Errorf("UnmarshalJSON() got = %v, want %v", got, tt.expected)
				}
			}
		})
	}
}

func TestTaskStatusResponse_UnmarshalJSON(t *testing.T) {
	// Test the exact format from the error log
	jsonStr := `{
		"task_id": "task_637bea604f4e",
		"session_id": "20260320-192416",
		"status": "completed",
		"created_at": "2026-03-20T19:26:05.277144",
		"updated_at": "2026-03-20T19:26:10.123456",
		"error_message": null,
		"histories_written": true
	}`

	var resp TaskStatusResponse
	err := json.Unmarshal([]byte(jsonStr), &resp)
	if err != nil {
		t.Errorf("UnmarshalJSON() unexpected error = %v", err)
		return
	}

	if resp.TaskID != "task_637bea604f4e" {
		t.Errorf("TaskID = %v, want %v", resp.TaskID, "task_637bea604f4e")
	}
	if resp.Status != "completed" {
		t.Errorf("Status = %v, want %v", resp.Status, "completed")
	}

	// Verify times were parsed (should not be zero)
	if resp.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if resp.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}
