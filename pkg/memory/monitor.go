package memory

import (
	"context"
	"time"

	"github.com/yourname/oxencode/pkg/config"
	"github.com/yourname/oxencode/pkg/logger"
)

// TaskMonitor 任务监控器
// 用于监控记忆服务的异步任务状态，在失败时自动重试
type TaskMonitor struct {
	client       *Client
	pollInterval time.Duration
	maxRetries   int
	timeout      time.Duration
	logger       logger.Logger
}

// NewTaskMonitor 从配置创建任务监控器
func NewTaskMonitor(client *Client, cfg *config.Config) *TaskMonitor {
	return &TaskMonitor{
		client:       client,
		pollInterval: time.Duration(cfg.MemoryMonitorPollInterval) * time.Second,
		maxRetries:   cfg.MemoryMonitorMaxRetries,
		timeout:      time.Duration(cfg.MemoryMonitorTimeout) * time.Second,
		logger:       logger.New("memory-monitor"),
	}
}

// StartMonitoring 启动异步监控（非阻塞）
// 启动goroutine后立即返回，监控在后台进行
func (m *TaskMonitor) StartMonitoring(ctx context.Context, taskID, sessionID string) {
	go m.monitorTask(ctx, taskID, sessionID, 0)
}

// monitorTask 监控单个任务
// 在goroutine中运行，定期轮询任务状态
func (m *TaskMonitor) monitorTask(ctx context.Context, taskID, sessionID string, retryCount int) {
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	// 创建轮询定时器
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	m.logger.Debug("Task monitoring started",
		"task_id", taskID,
		"session_id", sessionID,
		"retry_count", retryCount)

	for {
		select {
		case <-ctx.Done():
			m.logger.Warn("Task monitoring timeout",
				"task_id", taskID,
				"session_id", sessionID,
				"retry_count", retryCount)
			return

		case <-ticker.C:
			status, err := m.client.GetTaskStatus(ctx, taskID)
			if err != nil {
				m.logger.Error("Failed to get task status",
					"error", err,
					"task_id", taskID)
				// 继续轮询，不因为单次失败而退出
				continue
			}

			switch status.Status {
			case "completed":
				m.logger.Info("Task completed successfully",
					"task_id", taskID,
					"session_id", sessionID)
				return

			case "failed":
				if retryCount >= m.maxRetries {
					errMsg := ""
					if status.ErrorMessage != nil {
						errMsg = *status.ErrorMessage
					}
					m.logger.Error("Task failed, max retries exceeded",
						"task_id", taskID,
						"session_id", sessionID,
						"retry_count", retryCount,
						"error_message", errMsg)
					return
				}

				m.logger.Warn("Task failed, retrying...",
					"task_id", taskID,
					"session_id", sessionID,
					"attempt", retryCount+1)

				// 调用重试接口
				resp, err := m.client.RetrySession(ctx, sessionID)
				if err != nil {
					m.logger.Error("Retry request failed",
						"error", err,
						"session_id", sessionID)
					return
				}

				// 监控新任务（递归调用，增加重试计数）
				m.monitorTask(ctx, resp.TaskID, sessionID, retryCount+1)
				return

			case "pending", "running":
				// 任务进行中，继续等待
				m.logger.Debug("Task in progress",
					"task_id", taskID,
					"status", status.Status)
			}
		}
	}
}