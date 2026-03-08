package agent

import (
	"context"
	"fmt"
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
	"github.com/yourname/oxencode/pkg/prompt"
)

// Agent AI Agent 核心结构
type Agent struct {
	// 核心组件
	reactLoop    *ReActLoop            // 自实现的 ReAct 循环
	provider     fantasy.Provider      // fantasy Provider（多 Provider 支持）
	model        fantasy.LanguageModel // fantasy LanguageModel
	session      *ctxpkg.Session       // 上下文会话

	// 通用字段
	config       *config.Config
	toolRegistry *tools.Registry     // 工具注册表
	env          tools.Environment   // 执行环境
	logger       logger.Logger       // 日志记录器

	// Context Manager（可选，用于长程任务）
	ctxManager     ctxpkg.Manager
	currentSession *ctxpkg.Session

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

	// 创建 Session
	var session *ctxpkg.Session
	var compressor ctxpkg.Compressor

	// 尝试创建压缩器
	compressor, err = ctxpkg.NewLLMCompressorWithProvider(context.Background(), provider, cfg, log)
	if err != nil {
		log.Warn("Failed to create LLM compressor, using mock", "error", err)
		compressor = ctxpkg.NewMockCompressor(1024)
	}

	// 创建 Session
	session, err = ctxpkg.NewSession(&ctxpkg.SessionConfig{
		SystemPrompt: systemPrompt,
		MaxL1Pages:   10,
		Compressor:   compressor,
		Cfg:          cfg,
	})
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
	})

	return &Agent{
		reactLoop:    reactLoop,
		provider:     provider,
		model:        model,
		session:      session,
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

	log.Info("System prompt loaded", "source", promptDir, "length", len(p.SystemPrompt))
	return p.SystemPrompt
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

// ClearHistory 清空对话历史
func (a *Agent) ClearHistory() {
	// 创建新的空 Session
	if a.session != nil {
		a.session = nil
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

		// 设置回调
		a.reactLoop.SetCallbacks(&Callbacks{
			OnThought: func(text string) {
				select {
				case ch <- ProgressUpdate{Type: "thought", Content: text}:
				case <-ctx.Done():
				}
			},
			OnAction: func(toolName, input string) {
				// 工具调用时拍摄快照
				a.TakeSnapshot("tool_" + toolName)
				select {
				case ch <- ProgressUpdate{Type: "action", Content: input, ToolName: toolName}:
				case <-ctx.Done():
				}
			},
			OnObservation: func(toolName, output string) {
				// 工具结果返回后拍摄快照
				a.TakeSnapshot("result_" + toolName)
				select {
				case ch <- ProgressUpdate{Type: "observation", Content: output, ToolName: toolName}:
				case <-ctx.Done():
				}
			},
			OnContent: func(text string) {
				select {
				case ch <- ProgressUpdate{Type: "content", Content: text}:
				case <-ctx.Done():
				}
			},
			OnError: func(err error) {
				select {
				case ch <- ProgressUpdate{Type: "error", Content: err.Error()}:
				case <-ctx.Done():
				}
			},
			OnSnapshot: func(event string) {
				a.TakeSnapshot(event)
			},
		})

		// 流式执行
		for event := range a.reactLoop.Stream(ctx, userMessage) {
			select {
			case ch <- ProgressUpdate{
				Type:     event.Type,
				Content:  event.Content,
				ToolName: event.ToolName,
			}:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch
}


// InitContextManager 初始化上下文管理器
// 启用 context manager 后，Agent 将使用 session 管理上下文而非简单的 history 切片
func (a *Agent) InitContextManager(ctx context.Context, provider fantasy.Provider, archiveDir string) error {
	// 创建压缩器（仅用于L0）
	var compressor ctxpkg.Compressor
	var err error

	// 使用helper函数创建压缩器
	compressor, err = ctxpkg.NewLLMCompressorWithProvider(ctx, provider, a.config, a.logger)
	if err != nil {
		a.logger.Warn("Failed to create LLM compressor, using mock", "error", err)
		compressor = ctxpkg.NewMockCompressor(1024)
	}

	// 创建上下文管理器
	mgr, err := ctxpkg.NewManager(&ctxpkg.ManagerConfig{
		Compressor:    compressor,
		ArchiveDir:    archiveDir,
		DefaultPrompt: "", // 使用 Agent 现有的 system prompt
		Cfg:           a.config,
	})
	if err != nil {
		return fmt.Errorf("failed to create context manager: %w", err)
	}

	a.ctxManager = mgr

	// 创建默认 session
	session, err := mgr.NewSession(&ctxpkg.SessionConfig{
		SystemPrompt: a.reactLoop.systemPrompt, // 使用 ReActLoop 的系统提示词
		MaxL1Pages:   10,
		Compressor:   compressor,
	})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	a.currentSession = session
	a.logger.Info("Context manager initialized", "session_id", session.ID)
	return nil
}

// EnableContextManager 启用上下文管理器（如果已初始化）
func (a *Agent) EnableContextManager() bool {
	if a.ctxManager == nil {
		return false
	}
	return true
}

// CommitSession 提交当前 session 进行压缩
// 应该在每轮交互完成后调用
func (a *Agent) CommitSession(ctx context.Context) error {
	if a.currentSession == nil {
		return nil // 未启用 context manager
	}
	return a.currentSession.Commit(ctx)
}

// AddMessageToSession 添加消息到 session
func (a *Agent) AddMessageToSession(msg message.Message) {
	if a.currentSession == nil {
		return // 没有 session
	}
	a.currentSession.AddMessage(msg)
}

// GetContextMessages 获取上下文消息
// 如果启用了 context manager，返回 session 构建的上下文
func (a *Agent) GetContextMessages() []message.Message {
	if a.currentSession == nil {
		return []message.Message{}
	}
	return a.currentSession.GetContext()
}

// GetSessionStats 获取 session 统计信息
func (a *Agent) GetSessionStats() ctxpkg.SessionStats {
	if a.currentSession == nil {
		return ctxpkg.SessionStats{}
	}
	return a.currentSession.GetStats()
}

// NewSession 创建新 session
func (a *Agent) NewSession(systemPrompt string) (*ctxpkg.Session, error) {
	if a.ctxManager == nil {
		return nil, fmt.Errorf("context manager not initialized")
	}

	// 获取压缩器
	var compressor ctxpkg.Compressor
	compressor = ctxpkg.NewMockCompressor(1024) // 默认使用 mock

	session, err := a.ctxManager.NewSession(&ctxpkg.SessionConfig{
		SystemPrompt: systemPrompt,
		MaxL1Pages:   10,
		Compressor:   compressor,
	})
	if err != nil {
		return nil, err
	}

	a.currentSession = session
	a.logger.Info("New session created", "session_id", session.ID)
	return session, nil
}

// SwitchSession 切换到指定 session
func (a *Agent) SwitchSession(sessionID string) error {
	if a.ctxManager == nil {
		return fmt.Errorf("context manager not initialized")
	}

	session, err := a.ctxManager.GetSession(sessionID)
	if err != nil {
		return err
	}

	a.currentSession = session
	a.logger.Info("Session switched", "session_id", sessionID)
	return nil
}

// ListSessions 列出所有 session IDs
func (a *Agent) ListSessions() []string {
	if a.ctxManager == nil {
		return []string{}
	}
	return a.ctxManager.ListSessions()
}

// GetCurrentSessionID 获取当前 session ID
func (a *Agent) GetCurrentSessionID() string {
	if a.currentSession == nil {
		return ""
	}
	return a.currentSession.ID
}

// SearchArchive 搜索归档消息
func (a *Agent) SearchArchive(query string, limit int) ([]ctxarchive.ArchiveEntry, error) {
	// 这里可以扩展为通过 context.Manager 访问 archive.Manager
	// 简化版本返回空结果
	return []ctxarchive.ArchiveEntry{}, nil
}
