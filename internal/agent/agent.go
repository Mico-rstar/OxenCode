package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/fantasy"
	anthropic "charm.land/fantasy/providers/anthropic"
	azure "charm.land/fantasy/providers/azure"
	bedrock "charm.land/fantasy/providers/bedrock"
	google "charm.land/fantasy/providers/google"
	openai "charm.land/fantasy/providers/openai"
	openaicompat "charm.land/fantasy/providers/openaicompat"
	openrouter "charm.land/fantasy/providers/openrouter"
	vercel "charm.land/fantasy/providers/vercel"

	"github.com/yourname/oxencode/pkg/config"
	"github.com/yourname/oxencode/pkg/logger"
	"github.com/yourname/oxencode/internal/message"
	"github.com/yourname/oxencode/internal/prompt"
	"github.com/yourname/oxencode/internal/tools"
)

// Agent AI Agent 核心结构
type Agent struct {
	agent         fantasy.Agent
	config        *config.Config
	history       []message.Message // 对话历史
	toolRegistry  *tools.Registry   // 工具注册表
	env           tools.Environment // 执行环境
	logger        logger.Logger     // 日志记录器
}

// ProgressUpdate 进度更新类型
type ProgressUpdate struct {
	Type    string // "thought", "action", "observation", "content", "error", "done"
	Content string
	ToolName string // 仅用于 action 类型
}

// NewAgent 创建新的 Agent 实例
func NewAgent(cfg *config.Config) (*Agent, error) {
	// 获取 API Key
	apiKey := cfg.GetAPIKeyFromEnv()
	if apiKey == "" {
		return nil, fmt.Errorf("API key not found for provider %s: please set the appropriate environment variable", cfg.Provider)
	}

	// 创建 provider
	provider, err := createProvider(cfg, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	// 获取 language model
	model, err := provider.LanguageModel(context.Background(), cfg.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to get language model: %w", err)
	}

	// 创建 logger
	log := logger.New("agent")

	// 创建执行环境（MVP 版本使用本地环境）
	// 使用配置中的工作目录，如果未配置则使用当前目录
	workDir := cfg.WorkDir
	if workDir == "" {
		workDir = "."
	}
	env, err := tools.NewLocalEnvironment(workDir, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create environment: %w", err)
	}

	log.Info("Environment created", "workDir", workDir)

	// 创建工具注册表
	registry := tools.NewRegistry(log)

	// 注册 P0 工具
	globTool := tools.NewGlobTool(env, log)
	grepTool := tools.NewGrepTool(env, log)
	readTool := tools.NewReadTool(env, log)

	registry.Register(globTool)
	registry.Register(grepTool)
	registry.Register(readTool)

	// 注册 P1 工具（使用配置的超时时间）
	toolTimeout := time.Duration(cfg.ToolTimeout) * time.Second
	bashTool := tools.NewBashToolWithTimeout(env, log, toolTimeout)
	writeTool := tools.NewWriteTool(env, log)
	editTool := tools.NewEditTool(env, log)

	registry.Register(bashTool)
	registry.Register(writeTool)
	registry.Register(editTool)

	log.Info("Tools registered", "count", len(registry.Names()))

	// 将工具转换为 fantasy.AgentTool
	fantasyTools := tools.ToolsToAgentTools(registry.List())

	// 创建 Agent 并注册工具
	agent := fantasy.NewAgent(
		model,
		fantasy.WithMaxOutputTokens(int64(cfg.MaxTokens)),
		fantasy.WithTemperature(cfg.Temperature),
		fantasy.WithTools(fantasyTools...),
	)

	// 加载系统提示词
	systemPrompt := loadSystemPrompt(cfg, log)

	return &Agent{
		agent:        agent,
		config:       cfg,
		history: []message.Message{
			message.NewMessage(message.RoleSystem, systemPrompt),
		},
		toolRegistry: registry,
		env:          env,
		logger:       log,
	}, nil
}

// loadSystemPrompt 加载系统提示词
func loadSystemPrompt(cfg *config.Config, log logger.Logger) string {
	// 尝试从 prompt 目录加载
	promptDir := cfg.PromptDir
	if promptDir == "" {
		promptDir = "internal/prompt"
	}

	loader := prompt.NewLoader(promptDir)
	systemPrompt, err := loader.Load()
	if err != nil {
		log.Warn("Failed to load system prompt from file, using default", "error", err, "promptDir", promptDir)
		return getDefaultSystemPrompt()
	}

	log.Info("System prompt loaded", "source", promptDir, "length", len(systemPrompt))
	return systemPrompt
}

// getDefaultSystemPrompt 获取默认系统提示词
func getDefaultSystemPrompt() string {
	return "You are a helpful AI programming assistant."
}

// ReloadSystemPrompt 重新加载系统提示词
func (a *Agent) ReloadSystemPrompt() error {
	newPrompt := loadSystemPrompt(a.config, a.logger)
	a.SetSystemPrompt(newPrompt)
	a.logger.Info("System prompt reloaded")
	return nil
}

// createProvider 根据 provider 类型创建对应的 provider
func createProvider(cfg *config.Config, apiKey string) (fantasy.Provider, error) {
	switch cfg.Provider {
	case config.ProviderAnthropic:
		return anthropic.New(
			anthropic.WithAPIKey(apiKey),
		)

	case config.ProviderOpenAI:
		return openai.New(
			openai.WithAPIKey(apiKey),
		)

	case config.ProviderAzure:
		return azure.New(
			azure.WithBaseURL(cfg.AzureEndpoint),
			azure.WithAPIKey(apiKey),
			azure.WithAPIVersion(cfg.AzureAPIVersion),
		)

	case config.ProviderBedrock:
		return bedrock.New(
			bedrock.WithAPIKey(apiKey),
		)

	case config.ProviderGoogle:
		if cfg.GoogleLocation != "" {
			return google.New(
				google.WithVertex(cfg.GoogleProject, cfg.GoogleLocation),
			)
		}
		return google.New(
			google.WithGeminiAPIKey(apiKey),
		)

	case config.ProviderOpenAICompat:
		// 支持自定义 base_url
		if cfg.BaseURL != "" {
			return openaicompat.New(
				openaicompat.WithBaseURL(cfg.BaseURL),
				openaicompat.WithAPIKey(apiKey),
			)
		}
		return openaicompat.New(
			openaicompat.WithAPIKey(apiKey),
		)

	case config.ProviderQwen:
		// Qwen (通义千问) 使用 DashScope API
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
		}
		return openaicompat.New(
			openaicompat.WithBaseURL(baseURL),
			openaicompat.WithAPIKey(apiKey),
		)

	case config.ProviderDeepSeek:
		// DeepSeek API
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "https://api.deepseek.com"
		}
		return openaicompat.New(
			openaicompat.WithBaseURL(baseURL),
			openaicompat.WithAPIKey(apiKey),
		)

	case config.ProviderGLM:
		// GLM (智谱清言) API
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "https://open.bigmodel.cn/api/paas/v4"
		}
		return openaicompat.New(
			openaicompat.WithBaseURL(baseURL),
			openaicompat.WithAPIKey(apiKey),
		)

	case config.ProviderOpenRouter:
		return openrouter.New(
			openrouter.WithAPIKey(apiKey),
		)

	case config.ProviderVercel:
		return vercel.New(
			vercel.WithAPIKey(apiKey),
		)

	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}
}

// ChatStream 进行流式对话
func (a *Agent) ChatStream(ctx context.Context, userMessage string) (<-chan string, <-chan error) {
	streamCh := make(chan string)
	errCh := make(chan error, 1)

	go func() {
		defer close(streamCh)
		defer close(errCh)

		// 验证用户消息不为空
		if userMessage == "" {
			errCh <- fmt.Errorf("user message is empty")
			return
		}

		// 添加用户消息到历史
		userMsg := message.NewMessage(message.RoleUser, userMessage)
		a.history = append(a.history, userMsg)

		// 构建请求消息
		messages := a.buildMessages()

		// 验证构建的消息不为空
		if len(messages) == 0 {
			errCh <- fmt.Errorf("no messages to send")
			return
		}

		// 用于收集完整响应
		var fullResponse string

		// 调用流式 API
		result, err := a.agent.Stream(ctx, fantasy.AgentStreamCall{
			Prompt: userMessage,
			Messages: messages,
			OnTextDelta: func(id, delta string) error {
				fullResponse += delta
				streamCh <- delta
				return nil
			},
		})

		if err != nil {
			errCh <- fmt.Errorf("failed to call API: %w", err)
			return
		}

		_ = result // 避免未使用变量警告

		// 添加助手响应到历史
		assistantMsg := message.NewMessage(message.RoleAssistant, fullResponse)
		a.history = append(a.history, assistantMsg)
	}()

	return streamCh, errCh
}

// Chat 进行单轮对话（非流式）
func (a *Agent) Chat(ctx context.Context, userMessage string) (string, error) {
	// 添加用户消息到历史
	userMsg := message.NewMessage(message.RoleUser, userMessage)

	// 构建请求消息
	messages := a.buildMessages()

	// 验证消息不为空
	if len(messages) == 0 {
		return "", fmt.Errorf("no messages built from history")
	}

	// 调用 API
	result, err := a.agent.Generate(ctx, fantasy.AgentCall{
		Prompt: userMessage,
		Messages: messages,
	})

	if err != nil {
		return "", fmt.Errorf("failed to call API: %w", err)
	}

	a.history = append(a.history, userMsg)

	// 提取响应内容
	if result == nil {
		return "", fmt.Errorf("no response from API")
	}
	content := result.Response.Content.Text()

	// 添加助手消息到历史
	assistantMsg := message.NewMessage(message.RoleAssistant, content)
	a.history = append(a.history, assistantMsg)

	return content, nil
}

// buildMessages 构建请求消息
func (a *Agent) buildMessages() []fantasy.Message {
	messages := make([]fantasy.Message, 0, len(a.history))

	for _, msg := range a.history {
		switch msg.Role {
		case message.RoleUser:
			messages = append(messages, fantasy.NewUserMessage(msg.Content))
		case message.RoleAssistant:
			// Assistant 消息需要手动构建
			messages = append(messages, fantasy.Message{
				Role:    fantasy.MessageRoleAssistant,
				Content: []fantasy.MessagePart{fantasy.TextPart{Text: msg.Content}},
			})
		case message.RoleSystem:
			messages = append(messages, fantasy.NewSystemMessage(msg.Content))
		// RoleTool 暂时跳过，后续工具调用时实现
		}
	}

	return messages
}

// ClearHistory 清空对话历史
func (a *Agent) ClearHistory() {
	a.history = []message.Message{}
}

// GetHistory 获取对话历史
func (a *Agent) GetHistory() []message.Message {
	return a.history
}

// SetSystemPrompt 设置系统提示
func (a *Agent) SetSystemPrompt(prompt string) {
	// 移除旧的系统消息
	newHistory := make([]message.Message, 0, len(a.history))
	for _, msg := range a.history {
		if msg.Role != message.RoleSystem {
			newHistory = append(newHistory, msg)
		}
	}

	// 添加新的系统消息
	systemMsg := message.NewMessage(message.RoleSystem, prompt)
	newHistory = append([]message.Message{systemMsg}, newHistory...)

	a.history = newHistory
}

// ExecuteTool 执行工具调用
func (a *Agent) ExecuteTool(ctx context.Context, toolName string, input map[string]any) (string, error) {
	a.logger.Debug("Executing tool", "tool", toolName, "input", input)

	// 获取工具
	tool := a.toolRegistry.Get(toolName)
	if tool == nil {
		a.logger.Error("Tool not found", "tool", toolName)
		return "", fmt.Errorf("tool not found: %s", toolName)
	}

	// 参数验证
	if err := tool.Validate(input); err != nil {
		a.logger.Error("Parameter validation failed", "tool", toolName, "error", err)
		return "", fmt.Errorf("parameter validation failed: %w", err)
	}

	// 执行工具
	output, err := tool.Execute(ctx, input)
	if err != nil {
		a.logger.Error("Tool execution failed", "tool", toolName, "error", err)
		return "", fmt.Errorf("tool execution failed: %w", err)
	}

	a.logger.Info("Tool executed successfully", "tool", toolName, "outputLength", len(output))

	return output, nil
}

// GetToolSchemas 获取所有工具的 schema（用于传递给 LLM）
func (a *Agent) GetToolSchemas() []map[string]any {
	return a.toolRegistry.GetToolSchemas()
}

// GetEnvironment 获取执行环境
func (a *Agent) GetEnvironment() tools.Environment {
	return a.env
}

// GetToolRegistry 获取工具注册表
func (a *Agent) GetToolRegistry() *tools.Registry {
	return a.toolRegistry
}

// ChatWithTools 进行支持工具调用的对话（简化版 ReAct 循环）
// 这是一个同步方法，会自动处理工具调用直到完成
func (a *Agent) ChatWithTools(ctx context.Context, userMessage string) (string, error) {
	a.logger.Info("Starting ChatWithTools", "messageLength", len(userMessage))

	// 创建新的消息用于跟踪 ReAct 循环
	currentMsg := message.NewStreamingMessage(message.RoleAssistant)
	currentMsg.AddReActStep("thought", "User is asking: "+userMessage)

	// 添加用户消息到历史
	userMsg := message.NewMessage(message.RoleUser, userMessage)
	a.history = append(a.history, userMsg)

	// 开始 ReAct 循环
	maxIterations := 10 // 防止无限循环
	for iteration := 0; iteration < maxIterations; iteration++ {
		a.logger.Debug("ReAct iteration", "iteration", iteration+1, "max", maxIterations)

		// 构建消息列表，包含历史和工具调用结果
		messages := a.buildMessagesWithTools()

		// 获取工具名称列表
		toolNames := a.toolRegistry.Names()

		// 调用 LLM
		result, err := a.agent.Generate(ctx, fantasy.AgentCall{
			Prompt:       userMessage,
			Messages:     messages,
			ActiveTools: toolNames,
		})

		if err != nil {
			a.logger.Error("LLM generation failed", "error", err)
			currentMsg.SetError(err)
			return "", fmt.Errorf("LLM generation failed: %w", err)
		}

		// 提取响应内容
		content := result.Response.Content
		if len(content) == 0 {
			a.logger.Warn("LLM returned empty content")
			currentMsg.Complete()
			return "", nil
		}

		// 检查是否有工具调用
		hasToolCalls := false
		var responseText strings.Builder

		// 遍历响应内容
		for _, c := range content {
			switch content := c.(type) {
			case fantasy.TextContent:
				// 文本内容
				responseText.WriteString(content.Text)
				currentMsg.AppendContent(content.Text)

			case fantasy.ToolCallContent:
				// 工具调用
				hasToolCalls = true
				a.logger.Info("Tool call requested", "tool", content.ToolName, "callID", content.ToolCallID)

				// 添加 action 步骤
				currentMsg.AddToolCall(content.ToolName, map[string]any{})

				// 执行工具
				toolOutput, err := a.executeToolCall(ctx, content)
				if err != nil {
					a.logger.Error("Tool execution failed", "tool", content.ToolName, "error", err)
					currentMsg.UpdateToolCall(content.ToolName, err.Error(), message.StatusError, err.Error())

					// 添加错误观察
					currentMsg.AddReActStep("observation", fmt.Sprintf("Tool %s failed: %v", content.ToolName, err))

					// 将工具结果添加到历史（用于下一轮）
					toolResultMsg := message.NewMessage(message.RoleTool, fmt.Sprintf("Error: %v", err))
					a.history = append(a.history, toolResultMsg)
				} else {
					a.logger.Info("Tool executed successfully", "tool", content.ToolName, "outputLength", len(toolOutput))
					currentMsg.UpdateToolCall(content.ToolName, toolOutput, message.StatusCompleted, "")

					// 添加观察步骤
					// 限制观察内容长度
					observation := toolOutput
					if len(observation) > 500 {
						observation = observation[:500] + "\n... (truncated)"
					}
					currentMsg.AddReActStep("observation", observation)

					// 将工具结果添加到历史（用于下一轮）
					toolResultMsg := message.NewMessage(message.RoleTool, toolOutput)
					a.history = append(a.history, toolResultMsg)
				}
			}
		}

		// 如果没有工具调用，说明任务完成
		if !hasToolCalls {
			a.logger.Info("No tool calls, task completed")
			finalResponse := responseText.String()
			currentMsg.AppendContent(finalResponse)
			currentMsg.Complete()

			// 添加助手响应到历史
			assistantMsg := message.NewMessage(message.RoleAssistant, finalResponse)
			a.history = append(a.history, assistantMsg)

			return finalResponse, nil
		}

		// 有工具调用，继续下一轮循环
		a.logger.Debug("Continuing ReAct loop", "iteration", iteration+1)
	}

	a.logger.Warn("ReAct loop reached max iterations")
	return "", fmt.Errorf("ReAct loop reached maximum iterations (%d)", maxIterations)
}

// ChatWithToolsWithProgress 进行支持工具调用的对话（带进度更新）
// 返回一个 channel 用于异步推送进度更新和完成信号
// 进度更新类型：thought, action, observation, content, error, done
func (a *Agent) ChatWithToolsWithProgress(ctx context.Context, userMessage string) <-chan ProgressUpdate {
	ch := make(chan ProgressUpdate, 100)

	go func() {
		defer close(ch)

		a.logger.Info("Starting ChatWithToolsWithProgress", "messageLength", len(userMessage))

		// 辅助函数：发送进度更新
		sendProgress := func(updateType, content string, toolName string) {
			select {
			case ch <- ProgressUpdate{Type: updateType, Content: content, ToolName: toolName}:
			case <-ctx.Done():
			}
		}

		// 发送初始 thought
		sendProgress("thought", "User is asking: "+userMessage, "")

		// 添加用户消息到历史
		userMsg := message.NewMessage(message.RoleUser, userMessage)
		a.history = append(a.history, userMsg)

		// 构建消息列表
		messages := a.buildMessagesWithTools()

		// 获取工具名称列表
		toolNames := a.toolRegistry.Names()

		// 调用 LLM，使用 Stream 方法和回调来捕获工具调用事件
		result, err := a.agent.Stream(ctx, fantasy.AgentStreamCall{
			Prompt:       userMessage,
			Messages:     messages,
			ActiveTools: toolNames,
			// Tool execution callbacks
			OnToolCall: func(toolCall fantasy.ToolCallContent) error {
				// 解析工具输入
				var toolInput map[string]any
				if err := json.Unmarshal([]byte(toolCall.Input), &toolInput); err != nil {
					a.logger.Warn("Failed to parse tool input", "error", err)
					toolInput = map[string]any{}
				}

				// 格式化工具调用参数为函数调用形式，如 glob("**/*.go")
				inputStr := formatToolCall(toolCall.ToolName, toolInput)
				a.logger.Info("Tool call", "tool", toolCall.ToolName, "callID", toolCall.ToolCallID)
				sendProgress("action", inputStr, toolCall.ToolName)
				return nil
			},
			OnToolResult: func(result fantasy.ToolResultContent) error {
				// 提取工具执行结果
				var observation string
				var isError bool

				switch content := result.Result.(type) {
				case *fantasy.ToolResultOutputContentText:
					observation = content.Text
				case fantasy.ToolResultOutputContentText:
					observation = content.Text
				case *fantasy.ToolResultOutputContentError:
					observation = content.Error.Error()
					isError = true
				case fantasy.ToolResultOutputContentError:
					observation = content.Error.Error()
					isError = true
				case *fantasy.ToolResultOutputContentMedia:
					observation = content.Text
					if content.Data != "" {
						if observation != "" {
							observation += "\n"
						}
						observation += fmt.Sprintf("[Media: %s, %d bytes]", content.MediaType, len(content.Data))
					}
				case fantasy.ToolResultOutputContentMedia:
					observation = content.Text
					if content.Data != "" {
						if observation != "" {
							observation += "\n"
						}
						observation += fmt.Sprintf("[Media: %s, %d bytes]", content.MediaType, len(content.Data))
					}
				default:
					observation = fmt.Sprintf("%v", result.Result)
				}

				// 截断过长的观察结果
				if len(observation) > 500 {
					observation = observation[:500] + "\n... (truncated)"
				}

				a.logger.Info("Tool result", "toolName", result.ToolName, "isError", isError, "observationLength", len(observation))
				sendProgress("observation", observation, result.ToolName)
				return nil
			},
			OnFinish: func(result *fantasy.AgentResult) {
				a.logger.Info("Agent finished", "steps", len(result.Steps))
			},
		})

		if err != nil {
			a.logger.Error("LLM generation failed", "error", err)
			sendProgress("error", err.Error(), "")
			return
		}

		// 提取最终响应内容
		var finalText strings.Builder
		for _, c := range result.Response.Content {
			switch content := c.(type) {
			case fantasy.TextContent:
				finalText.WriteString(content.Text)
			}
		}

		finalResponse := finalText.String()

		// 添加助手响应到历史
		assistantMsg := message.NewMessage(message.RoleAssistant, finalResponse)
		a.history = append(a.history, assistantMsg)

		// 发送完成信号
		sendProgress("done", finalResponse, "")
	}()

	return ch
}

// ContinueReAct 继续执行 ReAct 循环（用于异步场景）
func (a *Agent) ContinueReAct(ctx context.Context, currentMsg message.Message) (string, error) {
	a.logger.Debug("ContinueReAct called")

	// 构建消息
	messages := a.buildMessagesWithTools()

	// 获取工具名称列表
	toolNames := a.toolRegistry.Names()

	// 调用 LLM（不需要 prompt，继续之前的对话）
	result, err := a.agent.Generate(ctx, fantasy.AgentCall{
		Messages:     messages,
		ActiveTools: toolNames,
	})

	if err != nil {
		a.logger.Error("ContinueReAct LLM call failed", "error", err)
		currentMsg.SetError(err)
		return "", err
	}

	// 处理响应
	content := result.Response.Content
	if len(content) == 0 {
		currentMsg.Complete()
		return "", nil
	}

	// 处理工具调用或文本响应
	hasToolCalls := false
	var responseText strings.Builder

	// 遍历响应内容
	for _, c := range content {
		switch content := c.(type) {
		case fantasy.TextContent:
			responseText.WriteString(content.Text)
			currentMsg.AppendContent(content.Text)

		case fantasy.ToolCallContent:
			hasToolCalls = true
			// 执行工具（与 ChatWithTools 相同的逻辑）
			toolOutput, err := a.executeToolCall(ctx, content)
			if err != nil {
				currentMsg.UpdateToolCall(content.ToolName, err.Error(), message.StatusError, err.Error())
				toolResultMsg := message.NewMessage(message.RoleTool, fmt.Sprintf("Error: %v", err))
				a.history = append(a.history, toolResultMsg)
			} else {
				currentMsg.UpdateToolCall(content.ToolName, toolOutput, message.StatusCompleted, "")
				toolResultMsg := message.NewMessage(message.RoleTool, toolOutput)
				a.history = append(a.history, toolResultMsg)
			}
		}
	}

	if !hasToolCalls {
		currentMsg.Complete()
		return responseText.String(), nil
	}

	// 还有工具调用，返回空字符串表示需要继续
	return "", nil
}

// executeToolCall 执行单个工具调用
func (a *Agent) executeToolCall(ctx context.Context, toolCall fantasy.ToolCallContent) (string, error) {
	// 解析工具输入参数
	var input map[string]any
	if err := json.Unmarshal([]byte(toolCall.Input), &input); err != nil {
		a.logger.Error("Failed to parse tool input", "error", err, "input", toolCall.Input)
		return "", fmt.Errorf("failed to parse tool input: %w", err)
	}

	// 执行工具
	output, err := a.ExecuteTool(ctx, toolCall.ToolName, input)
	if err != nil {
		return "", err
	}

	return output, nil
}

// buildMessagesWithTools 构建包含工具调用结果的消息列表
func (a *Agent) buildMessagesWithTools() []fantasy.Message {
	messages := make([]fantasy.Message, 0, len(a.history))

	for _, msg := range a.history {
		switch msg.Role {
		case message.RoleUser:
			messages = append(messages, fantasy.NewUserMessage(msg.Content))

		case message.RoleAssistant:
			// Assistant 消息需要手动构建
			messages = append(messages, fantasy.Message{
				Role:    fantasy.MessageRoleAssistant,
				Content: []fantasy.MessagePart{fantasy.TextPart{Text: msg.Content}},
			})

		case message.RoleSystem:
			messages = append(messages, fantasy.NewSystemMessage(msg.Content))

		case message.RoleTool:
			// 工具结果消息
			messages = append(messages, fantasy.Message{
				Role:    fantasy.MessageRoleUser, // 工具结果作为用户消息发送
				Content: []fantasy.MessagePart{fantasy.TextPart{Text: msg.Content}},
			})
		}
	}

	return messages
}

// formatToolCall 将工具调用格式化为函数调用形式，如 glob("**/*.go") 或 grep("pattern", "path")
func formatToolCall(toolName string, input map[string]any) string {
	if len(input) == 0 {
		return toolName + "()"
	}

	// 定义参数顺序（让常用参数显示在前面）
	paramOrder := map[string]int{
		"pattern":   0,
		"path":      1,
		"filePath":  1,
		"query":     0,
		"search":    0,
		"directory": 1,
	}

	// 按顺序排列参数
	type orderedParam struct {
		key   string
		value any
	}
	var params []orderedParam

	// 首先添加有定义顺序的参数
	for key, value := range input {
		if _, ok := paramOrder[key]; ok {
			params = append(params, orderedParam{key, value})
		}
	}
	// 按定义顺序排序
	sort.Slice(params, func(i, j int) bool {
		orderI, okI := paramOrder[params[i].key]
		orderJ, okJ := paramOrder[params[j].key]
		if !okI {
			orderI = 999
		}
		if !okJ {
			orderJ = 999
		}
		if orderI != orderJ {
			return orderI < orderJ
		}
		return params[i].key < params[j].key
	})

	// 然后添加其他参数
	for key, value := range input {
		if _, ok := paramOrder[key]; !ok {
			params = append(params, orderedParam{key, value})
		}
	}

	// 格式化参数
	var args []string
	for _, p := range params {
		argStr := formatValue(p.value)
		// 如果参数名不是常见的位置参数，显示参数名
		if _, ok := paramOrder[p.key]; !ok && len(params) > 1 {
			args = append(args, p.key+"="+argStr)
		} else {
			args = append(args, argStr)
		}
	}

	// 对于单个参数且是 pattern/query 等常见参数，简化显示
	if len(args) == 1 {
		if firstParam, ok := input["pattern"]; ok {
			return fmt.Sprintf("%s(%s)", toolName, formatValue(firstParam))
		}
		if firstParam, ok := input["query"]; ok {
			return fmt.Sprintf("%s(%s)", toolName, formatValue(firstParam))
		}
		if firstParam, ok := input["search"]; ok {
			return fmt.Sprintf("%s(%s)", toolName, formatValue(firstParam))
		}
	}

	return fmt.Sprintf("%s(%s)", toolName, strings.Join(args, ", "))
}

// formatValue 格式化参数值
func formatValue(v any) string {
	switch val := v.(type) {
	case string:
		// 字符串加引号，但如果太长则截断
		if len(val) > 50 {
			return fmt.Sprintf("\"%s...\"", val[:50])
		}
		return fmt.Sprintf("\"%s\"", val)
	case []any:
		// 数组格式化
		var parts []string
		for _, item := range val {
			parts = append(parts, formatValue(item))
		}
		return fmt.Sprintf("[%s]", strings.Join(parts, ", "))
	case map[string]any:
		// 对象简化显示
		if len(val) == 0 {
			return "{}"
		}
		var parts []string
		for k, kv := range val {
			parts = append(parts, fmt.Sprintf("%s: %s", k, formatValue(kv)))
		}
		return fmt.Sprintf("{%s}", strings.Join(parts, ", "))
	default:
		return fmt.Sprintf("%v", v)
	}
}

