package agent

import (
	"context"
	"fmt"

	"charm.land/fantasy"
	anthropic "charm.land/fantasy/providers/anthropic"
	azure "charm.land/fantasy/providers/azure"
	bedrock "charm.land/fantasy/providers/bedrock"
	google "charm.land/fantasy/providers/google"
	openai "charm.land/fantasy/providers/openai"
	openaicompat "charm.land/fantasy/providers/openaicompat"
	openrouter "charm.land/fantasy/providers/openrouter"
	vercel "charm.land/fantasy/providers/vercel"

	"github.com/yourname/oxencode/internal/config"
	"github.com/yourname/oxencode/internal/message"
)

// Agent AI Agent 核心结构
type Agent struct {
	agent   fantasy.Agent
	config  *config.Config
	history []message.Message // 对话历史
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

	// 创建 Agent (不在构造时设置系统提示)
	agent := fantasy.NewAgent(
		model,
		fantasy.WithMaxOutputTokens(int64(cfg.MaxTokens)),
		fantasy.WithTemperature(cfg.Temperature),
	)

	return &Agent{
		agent:   agent,
		config:  cfg,
		history: []message.Message{
			message.NewMessage(message.RoleSystem, "You are a helpful AI programming assistant."),
		},
	}, nil
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

