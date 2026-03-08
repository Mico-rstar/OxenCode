package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	ctxpkg "github.com/yourname/oxencode/internal/context"
	"github.com/yourname/oxencode/internal/message"
)

// Snapshot 上下文快照 - 包含与发送给LLM一致的完整消息视图
type Snapshot struct {
	Timestamp    string            `json:"timestamp"`
	Event        string            `json:"event"`
	SessionID    string            `json:"session_id"`
	TotalTokens  int               `json:"total_tokens"`
	Stats        SnapshotStats     `json:"stats"`
	Messages     []MessageSnapshot `json:"messages"`       // 完整的消息列表（与发送给LLM的一致）
	MessageCount int               `json:"message_count"`  // 消息总数
	CharCount    int               `json:"char_count"`     // 总字符数
}

// SnapshotStats 统计快照
type SnapshotStats struct {
	TotalL0Tokens int `json:"total_l0_tokens"`
	TotalL1Tokens int `json:"total_l1_tokens"`
	TotalL2Tokens int `json:"total_l2_tokens"`
	L1PageCount   int `json:"l1_page_count"`
	L2PageCount   int `json:"l2_page_count"`
}

// MessageSnapshot 消息快照
type MessageSnapshot struct {
	Index     int               `json:"index"`
	Role      string            `json:"role"`
	Content   string            `json:"content"`
	Length    int               `json:"length"`
	Truncated bool              `json:"truncated,omitempty"`
	ToolCalls []ToolCallSnapshot `json:"tool_calls,omitempty"`
}

// ToolCallSnapshot 工具调用快照
type ToolCallSnapshot struct {
	Name  string         `json:"name"`
	Input map[string]any `json:"input,omitempty"`
}

// SnapshotManager 快照管理器
type SnapshotManager struct {
	snapshotDir string
	enabled     bool
	lastUpdate  time.Time
}

// NewSnapshotManager 创建快照管理器
func NewSnapshotManager(snapshotDir string) *SnapshotManager {
	// 确保目录存在
	os.MkdirAll(snapshotDir, 0755)

	return &SnapshotManager{
		snapshotDir: snapshotDir,
		enabled:     true,
	}
}

// Enable 启用快照
func (sm *SnapshotManager) Enable() {
	sm.enabled = true
}

// Disable 禁用快照
func (sm *SnapshotManager) Disable() {
	sm.enabled = false
}

// TakeSnapshot 拍摄快照
func (sm *SnapshotManager) TakeSnapshot(session *ctxpkg.Session, event string) error {
	if !sm.enabled || session == nil {
		return nil
	}

	// 限制更新频率（最少间隔 50ms）
	if time.Since(sm.lastUpdate) < 50*time.Millisecond {
		return nil
	}
	sm.lastUpdate = time.Now()

	snapshot := sm.buildSnapshot(session, event)

	// 写入最新快照（覆盖）
	latestPath := filepath.Join(sm.snapshotDir, "latest.json")
	if err := sm.writeSnapshot(snapshot, latestPath); err != nil {
		return err
	}

	// 写入带时间戳的快照（保留历史）
	timestampedPath := filepath.Join(sm.snapshotDir,
		fmt.Sprintf("%s_%s.json", time.Now().Format("150405.000"), event))
	if err := sm.writeSnapshot(snapshot, timestampedPath); err != nil {
		return err
	}

	// 清理旧的快照（保留最近 100 个）
	sm.cleanupOldSnapshots(100)

	return nil
}

// buildSnapshot 构建快照（包含完整的消息视图）
func (sm *SnapshotManager) buildSnapshot(session *ctxpkg.Session, event string) *Snapshot {
	stats := session.GetStats()

	// 获取实际发送给 LLM 的上下文消息
	context := session.GetContext()

	snapshot := &Snapshot{
		Timestamp:   time.Now().Format("2006-01-02 15:04:05.000"),
		Event:       event,
		SessionID:   session.ID,
		TotalTokens: stats.TotalL0Tokens + stats.TotalL1Tokens + stats.TotalL2Tokens,
		Stats: SnapshotStats{
			TotalL0Tokens: stats.TotalL0Tokens,
			TotalL1Tokens: stats.TotalL1Tokens,
			TotalL2Tokens: stats.TotalL2Tokens,
			L1PageCount:   stats.L1PageCount,
			L2PageCount:   stats.L2PageCount,
		},
		Messages:     make([]MessageSnapshot, 0, len(context)),
		MessageCount: len(context),
		CharCount:    0,
	}

	// 构建完整的消息快照
	for i, msg := range context {
		msgSnap := MessageSnapshot{
			Index:   i,
			Role:    string(msg.Role),
			Content: msg.Content,
			Length:  len(msg.Content),
		}

		snapshot.CharCount += len(msg.Content)

		// 如果内容过长，标记为截断（但仍然保留完整内容）
		if len(msg.Content) > 10000 {
			msgSnap.Truncated = true
		}

		// 提取工具调用信息
		if len(msg.ReActLoop) > 0 {
			msgSnap.ToolCalls = make([]ToolCallSnapshot, 0)
			for _, step := range msg.ReActLoop {
				if step.ToolCall != nil {
					msgSnap.ToolCalls = append(msgSnap.ToolCalls, ToolCallSnapshot{
						Name:  step.ToolCall.Name,
						Input: step.ToolCall.Input,
					})
				}
			}
		}

		snapshot.Messages = append(snapshot.Messages, msgSnap)
	}

	return snapshot
}

// writeSnapshot 写入快照文件
func (sm *SnapshotManager) writeSnapshot(snapshot *Snapshot, path string) error {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write snapshot: %w", err)
	}

	return nil
}

// cleanupOldSnapshots 清理旧的快照文件
func (sm *SnapshotManager) cleanupOldSnapshots(keep int) {
	entries, err := os.ReadDir(sm.snapshotDir)
	if err != nil {
		return
	}

	// 过滤出带时间戳的快照（排除 latest.json）
	var snapshotFiles []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() && entry.Name() != "latest.json" {
			snapshotFiles = append(snapshotFiles, entry)
		}
	}

	// 如果快照数量超过保留数量，删除最旧的
	if len(snapshotFiles) > keep {
		for i := 0; i < len(snapshotFiles)-keep; i++ {
			path := filepath.Join(sm.snapshotDir, snapshotFiles[i].Name())
			os.Remove(path)
		}
	}
}

// ========================================
// Agent 快照方法
// ========================================

// TakeSnapshot 拍摄当前上下文快照
func (a *Agent) TakeSnapshot(event string) error {
	if a.snapshotManager == nil {
		return nil
	}
	return a.snapshotManager.TakeSnapshot(a.session, event)
}

// EnableSnapshot 启用快照功能
func (a *Agent) EnableSnapshot(snapshotDir string) {
	a.snapshotManager = NewSnapshotManager(snapshotDir)
	a.logger.Info("Snapshot enabled", "dir", snapshotDir)
}

// DisableSnapshot 禁用快照功能
func (a *Agent) DisableSnapshot() {
	if a.snapshotManager != nil {
		a.snapshotManager.Disable()
	}
}

// GetSnapshotManager 获取快照管理器
func (a *Agent) GetSnapshotManager() *SnapshotManager {
	return a.snapshotManager
}

// SnapshotOnMessage 消息添加时的快照钩子
func (a *Agent) SnapshotOnMessage(msg message.Message) {
	if a.snapshotManager == nil {
		return
	}

	event := fmt.Sprintf("msg_%s", msg.Role)
	a.snapshotManager.TakeSnapshot(a.session, event)
}

// SnapshotOnToolCall 工具调用时的快照钩子
func (a *Agent) SnapshotOnToolCall(toolName string) {
	if a.snapshotManager == nil {
		return
	}

	event := fmt.Sprintf("tool_%s", toolName)
	a.snapshotManager.TakeSnapshot(a.session, event)
}

// SnapshotOnCommit 提交时的快照钩子
func (a *Agent) SnapshotOnCommit() {
	if a.snapshotManager == nil {
		return
	}

	a.snapshotManager.TakeSnapshot(a.session, "commit")
}

// SnapshotOnSplit 分页时的快照钩子
func (a *Agent) SnapshotOnSplit() {
	if a.snapshotManager == nil {
		return
	}

	a.snapshotManager.TakeSnapshot(a.session, "split")
}