package context

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yourname/oxencode/internal/message"
	"github.com/yourname/oxencode/pkg/logger"
)

// CompressWorker 压缩工作器
// 负责管理压缩任务队列和执行压缩
type CompressWorker struct {
	queue      chan *CompressTask
	resultChan chan *CompressResult
	workerCount int
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	compressor Compressor
	logger     logger.Logger
}

// CompressTask 压缩任务
type CompressTask struct {
	Page     *Page
	Priority int // 优先级，数字越大优先级越高
}

// CompressResult 压缩结果
type CompressResult struct {
	PageID  PageID
	Content string
	Error   error
}

// CompressWorkerConfig 压缩工作器配置
type CompressWorkerConfig struct {
	WorkerCount int           // 并发工作器数量
	QueueSize   int           // 队列大小
	Timeout     time.Duration // 默认超时时间
}

// DefaultCompressWorkerConfig 返回默认配置
func DefaultCompressWorkerConfig() *CompressWorkerConfig {
	return &CompressWorkerConfig{
		WorkerCount: 2, // 默认 2 个工作器
		QueueSize:   50,
		Timeout:     60 * time.Second,
	}
}

// NewCompressWorker 创建压缩工作器
func NewCompressWorker(compressor Compressor, config *CompressWorkerConfig) *CompressWorker {
	ctx, cancel := context.WithCancel(context.Background())

	worker := &CompressWorker{
		queue:      make(chan *CompressTask, config.QueueSize),
		resultChan: make(chan *CompressResult, config.QueueSize),
		workerCount: config.WorkerCount,
		ctx:        ctx,
		cancel:     cancel,
		compressor: compressor,
		logger:     logger.New("context/worker"),
	}

	// 启动工作器
	worker.start()

	return worker
}

// start 启动工作器
func (w *CompressWorker) start() {
	w.logger.Info("Starting compress workers", "count", w.workerCount)

	for i := 0; i < w.workerCount; i++ {
		w.wg.Add(1)
		go w.workerLoop(i)
	}
}

// workerLoop 单个工作器循环
func (w *CompressWorker) workerLoop(id int) {
	defer w.wg.Done()

	w.logger.Debug("Worker started", "id", id)

	for {
		select {
		case <-w.ctx.Done():
			w.logger.Debug("Worker stopped", "id", id)
			return

		case task, ok := <-w.queue:
			if !ok {
				w.logger.Debug("Queue closed", "id", id)
				return
			}

			w.processTask(task, id)
		}
	}
}

// processTask 处理压缩任务
func (w *CompressWorker) processTask(task *CompressTask, workerID int) {
	w.logger.Info("Processing compress task",
		"worker_id", workerID,
		"page_id", task.Page.ID,
		"page_type", task.Page.Type,
		"priority", task.Priority,
	)

	// 创建带超时的上下文
	timeout := task.Page.Strategy.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second // 默认超时
	}

	ctx, cancel := context.WithTimeout(w.ctx, timeout)
	defer cancel()

	// 执行压缩
	content, err := w.compressor.Compress(ctx, joinMessages(task.Page.Messages), task.Page.Strategy)

	// 发送结果
	result := &CompressResult{
		PageID:  task.Page.ID,
		Content: content,
		Error:   err,
	}

	select {
	case w.resultChan <- result:
		w.logger.Debug("Compress result sent", "page_id", task.Page.ID)
	case <-w.ctx.Done():
		w.logger.Debug("Context cancelled, result discarded", "page_id", task.Page.ID)
	}
}

// joinMessages 将 messages 拼接为字符串
func joinMessages(messages []message.Message) string {
	var result string
	for _, msg := range messages {
		result += fmt.Sprintf("[%s] %s\n", msg.Role, msg.Content)
	}
	return result
}

// Submit 提交压缩任务
func (w *CompressWorker) Submit(page *Page, priority int) error {
	task := &CompressTask{
		Page:     page,
		Priority: priority,
	}

	select {
	case w.queue <- task:
		w.logger.Debug("Task submitted", "page_id", page.ID, "priority", priority)
		return nil
	case <-w.ctx.Done():
		return fmt.Errorf("worker stopped")
	default:
		return fmt.Errorf("queue full")
	}
}

// Results 返回结果 channel
func (w *CompressWorker) Results() <-chan *CompressResult {
	return w.resultChan
}

// Stop 停止工作器
func (w *CompressWorker) Stop() {
	w.logger.Info("Stopping compress workers")
	w.cancel()
	close(w.queue)
	w.wg.Wait()
	close(w.resultChan)
	w.logger.Info("All workers stopped")
}

// Stats 返回工作器统计
func (w *CompressWorker) Stats() CompressWorkerStats {
	return CompressWorkerStats{
		QueueLength: len(w.queue),
	}
}

// CompressWorkerStats 工作器统计
type CompressWorkerStats struct {
	QueueLength int `json:"queue_length"`
}
