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

// Run 执行 ReAct 循环（非流式）
func (r *ReActLoop) Run(ctx context.Context, userMessage string) (*RunResult, error) {
	// 1. 添加用户消息到 Session
	userMsg := message.NewMessage(message.RoleUser, userMessage)
	r.session.AddMessage(userMsg)

	// 2. ReAct 循环
	maxIterations := r.getMaxIterations()

	var result *RunResult
	var lastError error

	for i := 0; i < maxIterations; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// 3. 检查上下文长度，必要时压缩
		if err := r.checkAndCompress(ctx); err != nil {
			r.logger.Warn("Context compression failed", "error", err)
		}

		// 4. 构建消息（从 Session）
		messages := r.builder.Build(r.systemPrompt)

		// 5. 调用 LLM
		response, err := r.callLLM(ctx, messages)
		if err != nil {
			lastError = err
			if r.callbacks.OnError != nil {
				r.callbacks.OnError(err)
			}
			continue
		}

		// 6. 处理响应内容
		result = r.handleResponse(response)

		// 7. 检查是否有工具调用
		toolCalls := response.Content.ToolCalls()
		if len(toolCalls) == 0 {
			// 没有工具调用，返回最终结果
			if err := r.session.Commit(ctx); err != nil {
				r.logger.Warn("Failed to commit session", "error", err)
			}
			return result, nil
		}

		// 8. 执行工具调用
		for _, tc := range toolCalls {
			if err := r.executeToolCall(ctx, tc); err != nil {
				r.logger.Error("Tool execution failed", "tool", tc.ToolName, "error", err)
			}
		}

		// 9. 检查上下文长度（工具执行后），必要时分页
		r.checkAndSplit()
	}

	// 达到最大迭代次数
	if lastError != nil {
		return nil, fmt.Errorf("max iterations reached: %w", lastError)
	}
	return result, nil
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
		// 添加用户消息
		userMsg := message.NewMessage(message.RoleUser, userMessage)
		r.session.AddMessage(userMsg)

		// 初始快照：用户消息添加后
		r.snapshot("user_msg_added")

		maxIterations := r.getMaxIterations()

		for i := 0; i < maxIterations; i++ {
			select {
			case <-ctx.Done():
				yield(StreamEvent{Type: "error", Error: ctx.Err()})
				return
			default:
			}

			// 检查并压缩
			r.checkAndCompress(ctx)

			// 构建消息
			messages := r.builder.Build(r.systemPrompt)

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

						yield(StreamEvent{Type: "done", Content: finalContent.String()})
						return
					}
				}
			}

			// 处理流结束后仍有工具调用的情况
			if len(toolCalls) > 0 {
				// 执行工具
				for _, tc := range toolCalls {
					result, err := r.executor.Execute(ctx, tc)
					if err != nil {
						yield(StreamEvent{Type: "error", Error: err})
						return
					}

					// 添加工具结果到 Session
					toolMsg := message.NewMessage(message.RoleTool, result.Output)
					r.session.AddMessage(toolMsg)

					// 快照：工具执行后
					r.snapshot("tool_result_" + tc.ToolName)

					if r.callbacks.OnObservation != nil {
						r.callbacks.OnObservation(tc.ToolName, result.Output)
					}
					if !yield(StreamEvent{Type: "observation", Content: result.Output, ToolName: tc.ToolName}) {
						return
					}
				}

				// 检查分页
				r.checkAndSplit()

				// 快照：分页检查后
				r.snapshot("after_split")
			}
		}

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

// checkAndCompress 检查并触发压缩
func (r *ReActLoop) checkAndCompress(ctx context.Context) error {
	threshold := r.getContextCheckThreshold()
	currentTokens := r.session.GetStats().TotalL0Tokens + r.session.GetStats().TotalL1Tokens + r.session.GetStats().TotalL2Tokens

	if currentTokens > threshold {
		r.logger.Info("Context threshold reached, triggering compression",
			"current", currentTokens,
			"threshold", threshold)

		// 触发压缩（当前实现为分页）
		r.session.CheckAndSplit()
	}

	return nil
}

// checkAndSplit 检查并触发分页
func (r *ReActLoop) checkAndSplit() {
	threshold := r.getEmergencySplitThreshold()
	stats := r.session.GetStats()
	currentTokens := stats.TotalL0Tokens + stats.TotalL1Tokens + stats.TotalL2Tokens

	if currentTokens > threshold {
		r.logger.Warn("Emergency split threshold reached",
			"current", currentTokens,
			"threshold", threshold)

		r.session.CheckAndSplit()
	}
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

// getContextCheckThreshold 获取上下文检查阈值
func (r *ReActLoop) getContextCheckThreshold() int {
	// TODO: 从 config 读取
	return 80000
}

// getEmergencySplitThreshold 获取紧急分页阈值
func (r *ReActLoop) getEmergencySplitThreshold() int {
	// TODO: 从 config 读取
	return 100000
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