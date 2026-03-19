package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	"github.com/openai/openai-go/v2/option"

	ctxpkg "github.com/yourname/oxencode/internal/context"
	ctxarchive "github.com/yourname/oxencode/internal/context/archive"
	"github.com/yourname/oxencode/internal/message"
	"github.com/yourname/oxencode/internal/tools"
	"github.com/yourname/oxencode/pkg/config"
	"github.com/yourname/oxencode/pkg/logger"
	"github.com/yourname/oxencode/pkg/memory"
	"github.com/yourname/oxencode/pkg/prompt"
)

// Agent AI Agent 核心结构
type Agent struct {
	// 核心组件
	reactLoop    *ReActLoop            // 自实现的 ReAct 循环
	provider     fantasy.Provider      // fantasy Provider（多 Provider 支持）
	model        fantasy.LanguageModel // fantasy LanguageModel
	session      *ctxpkg.Session       // 上下文会话（直接持有）
	memoryClient *memory.Client        // 记忆服务客户端（可选）

	// 通用字段
	config       *config.Config
	toolRegistry *tools.Registry   // 工具注册表
	env          tools.Environment // 执行环境
	logger       logger.Logger     // 日志记录器

	// 快照管理器（用于调试和监控）
	snapshotManager *SnapshotManager
}

// ProgressUpdate 进度更新类型
type ProgressUpdate struct {
	Type     string // "thought", "action", "observation", "content", "error", "done"
	Content  string
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

	// 加载系统提示词
	systemPrompt := loadSystemPrompt(cfg, log)

	// 创建记忆服务客户端（如果启用）
	var memoryClient *memory.Client
	if cfg.MemoryEnabled {
		memoryClient = memory.NewClient(memory.DefaultClientConfig(cfg.MemoryServiceURL))
		log.Info("Memory service client created", "url", cfg.MemoryServiceURL)

		// 检查记忆服务健康状态
		if err := memoryClient.HealthCheck(context.Background()); err != nil {
			log.Warn("Memory service health check failed", "error", err)
		}

		// 注册记忆工具
		searchMemoryTool := tools.NewSearchMemoryTool(memoryClient, log)
		loadMemoryTool := tools.NewLoadMemoryTool(memoryClient, log)
		registry.Register(searchMemoryTool)
		registry.Register(loadMemoryTool)
		log.Info("Memory tools registered")
	}

	// 创建压缩器（仅用于L0）
	// 注意：使用独立的 provider，不设置 stream_options 等流式专用配置
	compressorProvider, err := createCompressorProvider(cfg, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create compressor provider: %w", err)
	}
	compressor, err := ctxpkg.NewLLMCompressorWithProvider(context.Background(), compressorProvider, cfg, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM compressor: %w", err)
	}

	// 直接创建 Session
	session, err := ctxpkg.NewSession(systemPrompt, cfg, compressor)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// 创建 ReActLoop
	reactLoop := NewReActLoop(&ReActLoopConfig{
		Model:        model,
		Session:      session,
		Registry:     registry,
		Env:          env,
		Config:       cfg,
		Logger:       log,
		SystemPrompt: systemPrompt,
		MemoryClient: memoryClient,
	})

	log.Info("Agent created", "session_id", session.ID)

	return &Agent{
		reactLoop:    reactLoop,
		provider:     provider,
		model:        model,
		session:      session,
		memoryClient: memoryClient,
		config:       cfg,
		toolRegistry: registry,
		env:          env,
		logger:       log,
	}, nil
}

// loadSystemPrompt 加载系统提示词
func loadSystemPrompt(cfg *config.Config, log logger.Logger) string {
	// 尝试从 prompt 目录加载
	promptDir := cfg.PromptDir

	p := prompt.New(promptDir)
	if err := p.Load(); err != nil {
		log.Warn("Failed to load system prompt from file, using default", "error", err, "promptDir", promptDir)
		panic("System prompt not found")
	}

	systemPrompt := p.SystemPrompt

	// 加载inner内容（如果记忆服务启用）
	if cfg.MemoryEnabled && cfg.MemoryDir != "" {
		innerContent := loadInnerContent(cfg.MemoryDir, log)
		if innerContent != "" {
			systemPrompt += "\n\n" + innerContent
			log.Info("Inner content loaded", "memory_dir", cfg.MemoryDir)
		}
	}

	log.Info("System prompt loaded", "source", promptDir, "length", len(systemPrompt))
	return systemPrompt
}

// loadInnerContent 加载inner目录内容（self.md和user.md）
func loadInnerContent(memoryDir string, log logger.Logger) string {
	var content strings.Builder

	// 加载 inner/self.md
	selfPath := filepath.Join(memoryDir, "inner", "self.md")
	if data, err := os.ReadFile(selfPath); err == nil && len(data) > 0 {
		content.WriteString("<self_cognition>\n")
		content.WriteString(string(data))
		content.WriteString("\n</self_cognition>\n")
		log.Debug("Loaded inner/self.md")
	}

	// 加载 inner/user.md
	userPath := filepath.Join(memoryDir, "inner", "user.md")
	if data, err := os.ReadFile(userPath); err == nil && len(data) > 0 {
		content.WriteString("<user_preference>\n")
		content.WriteString(string(data))
		content.WriteString("\n</user_preference>\n")
		log.Debug("Loaded inner/user.md")
	}

	return content.String()
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
			// 使用 WithSDKOptions 传递 extra_body 参数
			// 这相当于 Python 的 extra_body={"enable_thinking": True}
			openaicompat.WithSDKOptions(
				option.WithJSONSet("enable_thinking", true),
				// Python 的 stream_options={"include_usage": True}
				option.WithJSONSet("stream_options", map[string]any{
					"include_usage": true,
				}),
			),
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

// createCompressorProvider 创建用于压缩器的 provider
// 注意：不设置 stream_options 等流式专用配置，因为压缩器使用非流式调用（Generate）
func createCompressorProvider(cfg *config.Config, apiKey string) (fantasy.Provider, error) {
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
		return bedrock.New()

	case config.ProviderGoogle:
		return google.New(
			google.WithGeminiAPIKey(apiKey),
		)

	case config.ProviderOpenAICompat:
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
		// 注意：不设置 stream_options，因为压缩器使用非流式调用
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
		}
		return openaicompat.New(
			openaicompat.WithBaseURL(baseURL),
			openaicompat.WithAPIKey(apiKey),
		)

	case config.ProviderDeepSeek:
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "https://api.deepseek.com"
		}
		return openaicompat.New(
			openaicompat.WithBaseURL(baseURL),
			openaicompat.WithAPIKey(apiKey),
		)

	case config.ProviderGLM:
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

// ChatWithToolsWithProgress 进行支持工具调用的对话（带进度更新）
// 返回一个 channel 用于异步推送进度更新和完成信号
// 进度更新类型：thought, action, observation, content, error, done
func (a *Agent) ChatWithToolsWithProgress(ctx context.Context, userMessage string) <-chan ProgressUpdate {
	ch := make(chan ProgressUpdate, 100)

	go func() {
		defer close(ch)

		a.logger.Info("Starting ChatWithToolsWithProgress", "messageLength", len(userMessage))

		// 初始快照
		a.TakeSnapshot("start")

		// 设置快照回调（仅用于快照，不用于事件传递）
		a.reactLoop.SetCallbacks(&Callbacks{
			OnSnapshot: func(event string) {
				a.TakeSnapshot(event)
			},
		})

		// 流式执行（仅通过迭代器接收事件）
		for event := range a.reactLoop.Stream(ctx, userMessage) {
			// 处理 error 类型：将 Error 转换为 Content
			content := event.Content
			if event.Type == "error" && event.Error != nil {
				content = event.Error.Error()
			}
			select {
			case ch <- ProgressUpdate{
				Type:     event.Type,
				Content:  content,
				ToolName: event.ToolName,
			}:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch
}

// Close 关闭 Agent，释放资源
func (a *Agent) Close() {
	if a.session != nil {
		a.session.Close()
	}
	a.logger.Info("Agent closed")
}

// ClearSession 清空当前会话的对话历史
// 保留会话 ID 和配置，只清除对话内容
func (a *Agent) ClearSession() {
	if a.session != nil {
		a.session.Clear()
		a.logger.Info("Session cleared", "session_id", a.session.ID)
	}
}

// NewSession 创建新的会话
// 关闭当前会话并创建新会话
func (a *Agent) NewSession() error {
	// 关闭当前会话
	if a.session != nil {
		a.session.Close()
	}

	// 创建新会话
	systemPrompt := loadSystemPrompt(a.config, a.logger)

	// 创建压缩器
	apiKey := a.config.GetAPIKeyFromEnv()
	compressorProvider, err := createCompressorProvider(a.config, apiKey)
	if err != nil {
		return fmt.Errorf("failed to create compressor provider: %w", err)
	}
	compressor, err := ctxpkg.NewLLMCompressorWithProvider(context.Background(), compressorProvider, a.config, a.logger)
	if err != nil {
		return fmt.Errorf("failed to create LLM compressor: %w", err)
	}

	session, err := ctxpkg.NewSession(systemPrompt, a.config, compressor)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	a.session = session

	// 更新 ReActLoop 的 session
	a.reactLoop.session = session

	a.logger.Info("New session created", "session_id", session.ID)
	return nil
}

// GetSessionID 获取当前会话 ID
func (a *Agent) GetSessionID() string {
	if a.session != nil {
		return a.session.ID
	}
	return ""
}

// SearchArchive 搜索归档消息
func (a *Agent) SearchArchive(query string, limit int) ([]ctxarchive.ArchiveEntry, error) {
	// 简化版本返回空结果
	return []ctxarchive.ArchiveEntry{}, nil
}

// CommitSessionToMemory 将当前会话提交到记忆服务
// 返回task_id用于追踪异步处理状态
func (a *Agent) CommitSessionToMemory(ctx context.Context) (string, error) {
	if !a.config.MemoryEnabled {
		return "", fmt.Errorf("memory service not enabled")
	}

	if a.memoryClient == nil {
		return "", fmt.Errorf("memory client not initialized")
	}

	// 获取session中的所有消息
	messages := a.session.GetContext()
	if len(messages) == 0 {
		return "", fmt.Errorf("no messages to commit")
	}

	// 转换消息格式
	memMessages := make([]memory.MessageSchema, 0, len(messages))
	for _, msg := range messages {
		// 跳过系统消息
		if msg.Role == message.RoleSystem {
			continue
		}

		memMsg := memory.MessageSchema{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
		memMessages = append(memMessages, memMsg)
	}

	if len(memMessages) == 0 {
		return "", fmt.Errorf("no user/assistant messages to commit")
	}

	sessionID := a.session.ID

	resp, err := a.memoryClient.CommitSession(ctx, sessionID, memMessages)
	if err != nil {
		a.logger.Error("Failed to commit session to memory", "error", err)
		return "", err
	}

	a.logger.Info("Session committed to memory", "session_id", sessionID, "task_id", resp.TaskID)
	return resp.TaskID, nil
}

// IsMemoryEnabled 检查记忆服务是否启用
func (a *Agent) IsMemoryEnabled() bool {
	return a.config.MemoryEnabled && a.memoryClient != nil
}
