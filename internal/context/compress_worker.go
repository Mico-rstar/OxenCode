package context

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yourname/oxencode/internal/message"
)

// CompressWorker 压缩工作器
// 负责管理压缩任务队列和执行压缩
type CompressWorker struct {
	queue       chan *CompressTask
	resultChan  chan *CompressResult
	workerCount int
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	compressor  Compressor
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
		queue:       make(chan *CompressTask, config.QueueSize),
		resultChan:  make(chan *CompressResult, config.QueueSize),
		workerCount: config.WorkerCount,
		ctx:         ctx,
		cancel:      cancel,
		compressor:  compressor,
	}

	// 启动工作器
	worker.start()

	return worker
}

// start 启动工作器
func (w *CompressWorker) start() {
	for i := 0; i < w.workerCount; i++ {
		w.wg.Add(1)
		go w.workerLoop(i)
	}
}

// workerLoop 单个工作器循环
func (w *CompressWorker) workerLoop(id int) {
	defer w.wg.Done()

	for {
		select {
		case <-w.ctx.Done():
			return

		case task, ok := <-w.queue:
			if !ok {
				return
			}

			w.processTask(task, id)
		}
	}
}

// processTask 处理压缩任务
func (w *CompressWorker) processTask(task *CompressTask, workerID int) {
	// 创建带超时的上下文
	timeout := task.Page.Strategy.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second // 默认超时
	}

	ctx, cancel := context.WithTimeout(w.ctx, timeout)
	defer cancel()

	// 执行压缩
	// L0 压缩使用 Content（已预处理的内容），L1/L2 使用 Messages
	var inputContent string
	if task.Page.Type == PageTypeL0 && task.Page.Content != "" {
		inputContent = task.Page.Content
	} else {
		inputContent = joinMessages(task.Page.Messages)
	}

	content, err := w.compressor.Compress(ctx, inputContent, task.Page.Strategy)

	// 发送结果
	result := &CompressResult{
		PageID:  task.Page.ID,
		Content: content,
		Error:   err,
	}

	select {
	case w.resultChan <- result:
	case <-w.ctx.Done():
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
	w.cancel()
	close(w.queue)
	w.wg.Wait()
	close(w.resultChan)
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
