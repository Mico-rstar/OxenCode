package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"strings"

	"charm.land/fantasy"
	ctxpkg "github.com/yourname/oxencode/internal/context"
	"github.com/yourname/oxencode/internal/message"
	"github.com/yourname/oxencode/internal/tools"
	"github.com/yourname/oxencode/pkg/config"
	"github.com/yourname/oxencode/pkg/logger"
)

// ReActLoop 自实现的 ReAct 循环
// 完全控制消息流，实现与 Session 的深度集成
type ReActLoop struct {
	// 核心依赖
	model    fantasy.LanguageModel // 复用 fantasy 的模型接口
	session  *ctxpkg.Session       // 上下文管理
	registry *tools.Registry       // 工具注册表
	env      tools.Environment     // 执行环境

	// 配置
	cfg    *config.Config
	logger logger.Logger

	// 消息构建
	builder *MessageBuilder

	// 工具执行
	executor *ToolExecutor

	// 回调
	callbacks *Callbacks

	// 系统提示词
	systemPrompt string
}

// Callbacks 回调函数集合
type Callbacks struct {
	OnThought     func(text string)
	OnAction      func(toolName, input string)
	OnObservation func(toolName, output string)
	OnContent     func(text string)
	OnError       func(err error)

	// 快照回调
	OnSnapshot func(event string)
}

// ReActLoopConfig 配置
type ReActLoopConfig struct {
	Model        fantasy.LanguageModel
	Session      *ctxpkg.Session
	Registry     *tools.Registry
	Env          tools.Environment
	Config       *config.Config
	Logger       logger.Logger
	SystemPrompt string
}

// NewReActLoop 创建 ReAct 循环
func NewReActLoop(cfg *ReActLoopConfig) *ReActLoop {
	builder := NewMessageBuilder(cfg.Session, cfg.Config, cfg.Logger)
	executor := NewToolExecutor(cfg.Registry, cfg.Env, cfg.Config, cfg.Logger)

	return &ReActLoop{
		model:        cfg.Model,
		session:      cfg.Session,
		registry:     cfg.Registry,
		env:          cfg.Env,
		cfg:          cfg.Config,
		logger:       cfg.Logger,
		builder:      builder,
		executor:     executor,
		callbacks:    &Callbacks{},
		systemPrompt: cfg.SystemPrompt,
	}
}

// SetCallbacks 设置回调
func (r *ReActLoop) SetCallbacks(cb *Callbacks) {
	if cb != nil {
		r.callbacks = cb
	}
}

// SetSession 更新 Session 引用（用于切换 session 时）
func (r *ReActLoop) SetSession(session *ctxpkg.Session) {
	r.session = session
	// 同时更新 MessageBuilder 的 session 引用
	r.builder = NewMessageBuilder(session, r.cfg, r.logger)
}

// snapshot 拍摄快照
func (r *ReActLoop) snapshot(event string) {
	if r.callbacks != nil && r.callbacks.OnSnapshot != nil {
		r.callbacks.OnSnapshot(event)
	}
}

// RunResult 运行结果
type RunResult struct {
	Content      string
	FinishReason string
	Usage        fantasy.Usage
	Steps        int
}


// StreamEvent 流式事件
type StreamEvent struct {
	Type     string // "thought", "action", "observation", "content", "error", "done"
	Content  string
	ToolName string
	Error    error
}

// Stream 流式执行 ReAct 循环
func (r *ReActLoop) Stream(ctx context.Context, userMessage string) iter.Seq[StreamEvent] {
	return func(yield func(StreamEvent) bool) {
		// 添加用户消息（Session 会自动检查并触发压缩）
		userMsg := message.NewMessage(message.RoleUser, userMessage)
		r.session.AddMessage(userMsg)

		// 初始快照：用户消息添加后
		r.snapshot("user_msg_added")

		maxIterations := r.getMaxIterations()

		for i := 0; i < maxIterations; i++ {
			select {
			case <-ctx.Done():
				r.logger.Warn("Context cancelled", "error", ctx.Err())
				yield(StreamEvent{Type: "error", Error: ctx.Err()})
				return
			default:
			}

			// 构建消息
			messages := r.builder.Build()

			// Debug: 打印消息详情
			r.logger.Debug("Built messages for LLM", "count", len(messages))
			for i, msg := range messages {
				hasToolCalls := false
				hasToolResult := false
				for _, part := range msg.Content {
					switch part.(type) {
					case fantasy.ToolCallPart:
						hasToolCalls = true
					case fantasy.ToolResultPart:
						hasToolResult = true
					}
				}
				if hasToolCalls || hasToolResult || msg.Role == fantasy.MessageRoleUser {
					r.logger.Debug("Message detail", "index", i, "role", msg.Role, "has_tool_calls", hasToolCalls, "has_tool_result", hasToolResult)
				}
			}

			// 快照：每次 LLM 调用前
			r.snapshot(fmt.Sprintf("llm_call_iter_%d", i))

			// 流式调用 LLM
			stream, err := r.model.Stream(ctx, fantasy.Call{
				Prompt:          messages,
				MaxOutputTokens: ptr(int64(r.cfg.MaxTokens)),
				Temperature:     ptr(r.cfg.Temperature),
				Tools:           r.buildToolDefinitions(),
			})
			if err != nil {
				r.logger.Error("Failed to call LLM stream", "error", err)
				yield(StreamEvent{Type: "error", Error: err})
				return
			}

			// 处理流式响应
			var toolCalls []fantasy.ToolCallContent
			var finalContent strings.Builder
			var reasoningContent strings.Builder

			for part := range stream {
				// 处理错误
				if part.Error != nil {
					r.logger.Error("LLM stream error", "error", part.Error)
					yield(StreamEvent{Type: "error", Error: part.Error})
					return
				}

				// 根据 StreamPart.Type 处理不同类型
				switch part.Type {
				case fantasy.StreamPartTypeTextDelta:
					finalContent.WriteString(part.Delta)
					if r.callbacks.OnContent != nil {
						r.callbacks.OnContent(part.Delta)
					}
					if !yield(StreamEvent{Type: "content", Content: part.Delta}) {
						return
					}

				case fantasy.StreamPartTypeReasoningDelta:
					reasoningContent.WriteString(part.Delta)
					if r.callbacks.OnThought != nil {
						r.callbacks.OnThought(part.Delta)
					}
					if !yield(StreamEvent{Type: "thought", Content: part.Delta}) {
						return
					}

				case fantasy.StreamPartTypeToolCall:
					// 构建工具调用
					tc := fantasy.ToolCallContent{
						ToolCallID: part.ID,
						ToolName:   part.ToolCallName,
						Input:      part.ToolCallInput,
					}
					toolCalls = append(toolCalls, tc)

					inputStr := formatToolInput(tc.ToolName, tc.Input)
					if r.callbacks.OnAction != nil {
						r.callbacks.OnAction(tc.ToolName, inputStr)
					}
					if !yield(StreamEvent{Type: "action", Content: inputStr, ToolName: tc.ToolName}) {
						return
					}

				case fantasy.StreamPartTypeFinish:
					// 流式完成，检查是否有工具调用
					if len(toolCalls) == 0 {
						// 添加最终响应到 Session
						assistantMsg := message.NewMessage(message.RoleAssistant, finalContent.String())
						r.session.AddMessage(assistantMsg)

						if err := r.session.Commit(ctx); err != nil {
							r.logger.Warn("Failed to commit session", "error", err)
						}

						// done 事件不携带内容，调用者已通过流式 content 收到所有内容
						yield(StreamEvent{Type: "done"})
						return
					}
				}
			}

			// 处理流结束后仍有工具调用的情况
			if len(toolCalls) > 0 {
				// 创建原子序列：assistant + tool results
				atom := message.NewAtomSequence()

				// 创建 Assistant 消息，包含工具调用信息
				assistantMsg := message.NewMessage(message.RoleAssistant, finalContent.String())
				for _, tc := range toolCalls {
					// 解析工具输入
					var inputMap map[string]any
					if tc.Input != "" {
						if err := json.Unmarshal([]byte(tc.Input), &inputMap); err != nil {
							r.logger.Warn("Failed to parse tool input", "tool", tc.ToolName, "error", err)
							inputMap = make(map[string]any)
						}
					} else {
						inputMap = make(map[string]any)
					}
					// 使用 LLM 返回的 ToolCallID，而不是生成新的
					assistantMsg.AddToolCallWithID(tc.ToolCallID, tc.ToolName, inputMap)
				}
				atom.SetAssistant(assistantMsg)

				// 执行工具并收集结果
				for _, tc := range toolCalls {
					result, err := r.executor.Execute(ctx, tc)
					if err != nil {
						r.logger.Error("Tool execution failed", "tool", tc.ToolName, "error", err)
						yield(StreamEvent{Type: "error", Error: err})
						return
					}

					// 添加工具结果到原子序列
					toolMsg := message.NewToolResultMessage(tc.ToolCallID, result.Output)
					atom.AddToolResult(toolMsg)

					// 快照：工具执行后
					r.snapshot("tool_result_" + tc.ToolName)

					if r.callbacks.OnObservation != nil {
						r.callbacks.OnObservation(tc.ToolName, result.Output)
					}
					if !yield(StreamEvent{Type: "observation", Content: result.Output, ToolName: tc.ToolName}) {
						return
					}
				}

				// 原子性地添加整个序列到 Session
				r.session.AddAtom(atom)
			}
		}

		r.logger.Warn("Max iterations reached", "max", maxIterations)
		yield(StreamEvent{Type: "error", Error: fmt.Errorf("max iterations reached")})
	}
}

// callLLM 调用 LLM（非流式）
func (r *ReActLoop) callLLM(ctx context.Context, messages []fantasy.Message) (*fantasy.Response, error) {
	return r.model.Generate(ctx, fantasy.Call{
		Prompt:          messages,
		MaxOutputTokens: ptr(int64(r.cfg.MaxTokens)),
		Temperature:     ptr(r.cfg.Temperature),
		Tools:           r.buildToolDefinitions(),
	})
}

// handleResponse 处理响应
func (r *ReActLoop) handleResponse(response *fantasy.Response) *RunResult {
	return &RunResult{
		Content:      response.Content.Text(),
		FinishReason: string(response.FinishReason),
		Usage:        response.Usage,
	}
}

// executeToolCall 执行工具调用
func (r *ReActLoop) executeToolCall(ctx context.Context, tc fantasy.ToolCallContent) error {
	// 执行工具
	result, err := r.executor.Execute(ctx, tc)
	if err != nil {
		return err
	}

	// 添加工具结果到 Session
	toolMsg := message.NewMessage(message.RoleTool, result.Output)
	r.session.AddMessage(toolMsg)

	// 触发回调
	if r.callbacks.OnObservation != nil {
		r.callbacks.OnObservation(tc.ToolName, result.Output)
	}

	return nil
}

// buildToolDefinitions 构建工具定义
func (r *ReActLoop) buildToolDefinitions() []fantasy.Tool {
	toolList := r.registry.List()
	result := make([]fantasy.Tool, len(toolList))
	for i, tool := range toolList {
		// 解析 JSON Schema
		var inputSchema map[string]any
		if err := json.Unmarshal(tool.Parameters(), &inputSchema); err != nil {
			r.logger.Warn("Failed to parse tool parameters", "tool", tool.Name(), "error", err)
			inputSchema = make(map[string]any)
		}

		// 使用 fantasy.FunctionTool 实现 fantasy.Tool 接口
		result[i] = fantasy.FunctionTool{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: inputSchema,
		}
	}
	return result
}

// getMaxIterations 获取最大迭代次数
func (r *ReActLoop) getMaxIterations() int {
	// TODO: 从 config 读取
	return 50
}

// ptr 辅助函数：创建指针
func ptr[T any](v T) *T {
	return &v
}

// formatToolInput 格式化工具输入
func formatToolInput(toolName, input string) string {
	if input == "" {
		return toolName + "()"
	}
	return fmt.Sprintf("%s(%s)", toolName, input)
}