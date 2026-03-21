package memory

import (
	"strings"
	"time"
)

// FlexibleTime 支持多种时间格式的反序列化
type FlexibleTime struct {
	time.Time
}

// UnmarshalJSON 实现 json.Unmarshaler 接口，支持多种时间格式
func (ft *FlexibleTime) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), "\"")
	if s == "" {
		return nil
	}

	// 尝试多种时间格式
	formats := []string{
		time.RFC3339,                    // 2006-01-02T15:04:05Z07:00
		time.RFC3339Nano,                // 2006-01-02T15:04:05.999999999Z07:00
		"2006-01-02T15:04:05.999999",   // Python isoformat() 无时区
		"2006-01-02T15:04:05",          // 无时区无微秒
		"2006-01-02 15:04:05",          // 空格分隔
		"2006-01-02",                   // 仅日期
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			// 如果原始格式没有时区信息，假设为 UTC
			if t.Location().String() == "" {
				ft.Time = t.UTC()
			} else {
				ft.Time = t
			}
			return nil
		}
	}

	return nil
}

// TriggerMemoryRequest 快速判断是否有相关记忆
type TriggerMemoryRequest struct {
	Query     string  `json:"query"`
	Threshold float64 `json:"threshold,omitempty"`
}

// TriggerMemoryResponse trigger_memory响应
type TriggerMemoryResponse struct {
	HasRelevant bool    `json:"has_relevant"`
	Hint        string  `json:"hint,omitempty"`
	Score       float64 `json:"score"`
}

// SearchMemoryRequest RAG检索请求
type SearchMemoryRequest struct {
	Queries []string `json:"queries"`
	TopK    int      `json:"top_k"`
	Types   []string `json:"types,omitempty"`
}

// MemoryResult 单个记忆搜索结果
type MemoryResult struct {
	ID          string  `json:"id"`
	Description string  `json:"description"`
	Score       float64 `json:"score"`
	Excerpt     string  `json:"excerpt"`
}

// SearchMemoryResponse search_memory响应
type SearchMemoryResponse struct {
	Results []MemoryResult `json:"results"`
}

// LoadMemoryRequest 加载完整记忆内容请求
type LoadMemoryRequest struct {
	IDs []string `json:"ids"`
}

// MemoryContent 完整记忆内容
type MemoryContent struct {
	ID          string  `json:"id"`
	Content     string  `json:"content"`
	Source      string  `json:"source"`
	Description *string `json:"description,omitempty"`
}

// LoadMemoryResponse load_memory响应
type LoadMemoryResponse struct {
	Memories []MemoryContent `json:"memories"`
}

// MessageSchema 消息结构（用于commit_session）
type MessageSchema struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	Timestamp *time.Time `json:"timestamp,omitempty"`
}

// CommitSessionRequest 提交session请求
type CommitSessionRequest struct {
	SessionID string          `json:"session_id"`
	Messages  []MessageSchema `json:"messages"`
}

// CommitSessionResponse commit_session响应
type CommitSessionResponse struct {
	TaskID string `json:"task_id"`
}

// TaskStatusResponse 任务状态响应
type TaskStatusResponse struct {
	TaskID           string      `json:"task_id"`
	SessionID        string      `json:"session_id"`
	Status           string      `json:"status"`
	CreatedAt        FlexibleTime `json:"created_at"`
	UpdatedAt        FlexibleTime `json:"updated_at"`
	ErrorMessage     *string     `json:"error_message,omitempty"`
	HistoriesWritten bool        `json:"histories_written"`
}

// RetrySessionRequest retry_session请求
type RetrySessionRequest struct {
	SessionID string `json:"session_id"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error  string  `json:"error"`
	Detail *string `json:"detail,omitempty"`
}