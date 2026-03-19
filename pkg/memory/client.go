package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/yourname/oxencode/pkg/logger"
)

// Client 记忆服务HTTP客户端
type Client struct {
	baseURL    string
	httpClient *http.Client
	logger     logger.Logger
}

// ClientConfig 客户端配置
type ClientConfig struct {
	BaseURL        string
	Timeout        time.Duration
	MaxRetries     int
	RetryInterval  time.Duration
}

// DefaultClientConfig 默认配置
func DefaultClientConfig(baseURL string) *ClientConfig {
	return &ClientConfig{
		BaseURL:       baseURL,
		Timeout:       30 * time.Second,
		MaxRetries:    3,
		RetryInterval: 1 * time.Second,
	}
}

// NewClient 创建新的记忆服务客户端
func NewClient(cfg *ClientConfig) *Client {
	if cfg == nil {
		cfg = DefaultClientConfig("http://127.0.0.1:8765")
	}

	return &Client{
		baseURL: cfg.BaseURL,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		logger: logger.New("memory-client"),
	}
}

// doRequest 执行HTTP请求，带重试
func (c *Client) doRequest(ctx context.Context, method, path string, reqBody, respBody any) error {
	var body io.Reader
	if reqBody != nil {
		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		body = bytes.NewReader(jsonData)
	}

	url := c.baseURL + path

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			c.logger.Warn("Retrying request", "attempt", attempt, "path", path)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second * time.Duration(attempt)):
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}
		defer resp.Body.Close()

		respData, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		if resp.StatusCode >= 400 {
			var errResp ErrorResponse
			if json.Unmarshal(respData, &errResp) == nil {
				lastErr = fmt.Errorf("API error %d: %s", resp.StatusCode, errResp.Error)
			} else {
				lastErr = fmt.Errorf("API error %d: %s", resp.StatusCode, string(respData))
			}
			continue
		}

		if respBody != nil {
			if err := json.Unmarshal(respData, respBody); err != nil {
				return fmt.Errorf("failed to unmarshal response: %w", err)
			}
		}

		return nil
	}

	return lastErr
}

// TriggerMemory 快速判断是否有相关记忆
func (c *Client) TriggerMemory(ctx context.Context, query string) (*TriggerMemoryResponse, error) {
	req := &TriggerMemoryRequest{
		Query:     query,
		Threshold: 0.7,
	}
	resp := &TriggerMemoryResponse{}
	if err := c.doRequest(ctx, http.MethodPost, "/trigger_memory", req, resp); err != nil {
		c.logger.Error("TriggerMemory failed", "error", err, "query", query)
		return nil, err
	}
	return resp, nil
}

// SearchMemory RAG检索记忆
func (c *Client) SearchMemory(ctx context.Context, queries []string, topK int) (*SearchMemoryResponse, error) {
	req := &SearchMemoryRequest{
		Queries: queries,
		TopK:    topK,
	}
	resp := &SearchMemoryResponse{}
	if err := c.doRequest(ctx, http.MethodPost, "/search_memory", req, resp); err != nil {
		c.logger.Error("SearchMemory failed", "error", err, "queries", queries)
		return nil, err
	}
	return resp, nil
}

// LoadMemory 加载完整记忆内容
func (c *Client) LoadMemory(ctx context.Context, ids []string) (*LoadMemoryResponse, error) {
	req := &LoadMemoryRequest{
		IDs: ids,
	}
	resp := &LoadMemoryResponse{}
	if err := c.doRequest(ctx, http.MethodPost, "/load_memory", req, resp); err != nil {
		c.logger.Error("LoadMemory failed", "error", err, "ids", ids)
		return nil, err
	}
	return resp, nil
}

// CommitSession 提交session进行异步处理
func (c *Client) CommitSession(ctx context.Context, sessionID string, messages []MessageSchema) (*CommitSessionResponse, error) {
	req := &CommitSessionRequest{
		SessionID: sessionID,
		Messages:  messages,
	}
	resp := &CommitSessionResponse{}
	if err := c.doRequest(ctx, http.MethodPost, "/commit_session", req, resp); err != nil {
		c.logger.Error("CommitSession failed", "error", err, "session_id", sessionID)
		return nil, err
	}
	c.logger.Info("Session committed", "session_id", sessionID, "task_id", resp.TaskID)
	return resp, nil
}

// GetTaskStatus 获取任务状态
func (c *Client) GetTaskStatus(ctx context.Context, taskID string) (*TaskStatusResponse, error) {
	resp := &TaskStatusResponse{}
	if err := c.doRequest(ctx, http.MethodGet, "/task/"+taskID+"/status", nil, resp); err != nil {
		c.logger.Error("GetTaskStatus failed", "error", err, "task_id", taskID)
		return nil, err
	}
	return resp, nil
}

// RetrySession 重试失败的session处理
func (c *Client) RetrySession(ctx context.Context, sessionID string) (*CommitSessionResponse, error) {
	req := &RetrySessionRequest{
		SessionID: sessionID,
	}
	resp := &CommitSessionResponse{}
	if err := c.doRequest(ctx, http.MethodPost, "/retry_session", req, resp); err != nil {
		c.logger.Error("RetrySession failed", "error", err, "session_id", sessionID)
		return nil, err
	}
	c.logger.Info("Session retry initiated", "session_id", sessionID, "task_id", resp.TaskID)
	return resp, nil
}

// HealthCheck 检查服务健康状态
func (c *Client) HealthCheck(ctx context.Context) error {
	resp := struct {
		Status string `json:"status"`
	}{}
	if err := c.doRequest(ctx, http.MethodGet, "/health", nil, &resp); err != nil {
		return err
	}
	if resp.Status != "ok" {
		return fmt.Errorf("service unhealthy: %s", resp.Status)
	}
	return nil
}