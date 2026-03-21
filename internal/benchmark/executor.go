package benchmark

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/fantasy"
	anthropic "charm.land/fantasy/providers/anthropic"
	openaicompat "charm.land/fantasy/providers/openaicompat"

	ctxpkg "github.com/yourname/oxencode/internal/context"
	"github.com/yourname/oxencode/internal/message"
	"github.com/yourname/oxencode/internal/tools"
	"github.com/yourname/oxencode/pkg/config"
	"github.com/yourname/oxencode/pkg/logger"
)

// Executor 基准测试执行器
type Executor struct {
	config       *BenchmarkConfig
	loader       *ScenarioLoader
	memController MemoryController
	logger       logger.Logger

	// Agent 组件复用
	provider     fantasy.Provider
	toolRegistry *tools.Registry
}

// NewExecutor 创建执行器
func NewExecutor(cfg *BenchmarkConfig) *Executor {
	log := logger.New("benchmark")
	return &Executor{
		config: cfg,
		loader: NewScenarioLoader("internal/benchmark/scenarios"),
		logger: log,
	}
}

// RunScenario 执行单个场景
func (e *Executor) RunScenario(ctx context.Context, scenario *BenchmarkScenario, memoryEnabled bool) (*BenchmarkResult, error) {
	e.logger.Info("Running scenario", "id", scenario.ID, "memory_enabled", memoryEnabled)

	startTime := time.Now()
	result := &BenchmarkResult{
		ScenarioID:    scenario.ID,
		Dimension:     scenario.Dimension,
		Timestamp:     startTime,
		MemoryEnabled: memoryEnabled,
	}

	// 1. 初始化记忆控制器
	memCtrl, err := NewFileMemoryController(e.config.MemoryServiceURL)
	if err != nil {
		return nil, fmt.Errorf("创建记忆控制器失败: %w", err)
	}
	e.memController = memCtrl

	// 2. 重置记忆状态
	if err := memCtrl.Reset(ctx); err != nil {
		return nil, fmt.Errorf("重置记忆状态失败: %w", err)
	}

	// 3. 如果启用记忆且场景有记忆预置条件，注入记忆
	if memoryEnabled && len(scenario.MemoryPrecondition.Experience) > 0 ||
		len(scenario.MemoryPrecondition.Knowledge) > 0 {
		if err := memCtrl.InjectMemories(ctx, &scenario.MemoryPrecondition); err != nil {
			return nil, fmt.Errorf("注入记忆失败: %w", err)
		}
	}

	// 4. 创建 Agent 配置
	agentCfg := e.createAgentConfig(scenario, memoryEnabled)

	// 5. 创建 Agent 组件
	session, reactLoop, err := e.createAgentComponents(ctx, agentCfg)
	if err != nil {
		return nil, fmt.Errorf("创建 Agent 组件失败: %w", err)
	}

	// 6. 执行会话
	var sessionResults []SessionResult
	for i, sessionSpec := range scenario.Sessions {
		sessionResult, err := e.executeSession(ctx, session, reactLoop, &sessionSpec, i)
		if err != nil {
			result.Error = err.Error()
			break
		}
		sessionResults = append(sessionResults, *sessionResult)

		// 会话间等待（用于记忆处理）
		if sessionSpec.WaitAfter > 0 {
			time.Sleep(time.Duration(sessionSpec.WaitAfter) * time.Second)
		}
	}

	result.Sessions = sessionResults
	result.Duration = time.Since(startTime)

	// 7. 计算指标
	result.Metrics = e.calculateMetrics(sessionResults, scenario)

	return result, nil
}

// RunComparison 执行对比测试
func (e *Executor) RunComparison(ctx context.Context, scenario *BenchmarkScenario) (*ComparisonResult, error) {
	e.logger.Info("Running comparison", "scenario_id", scenario.ID)

	// 运行无记忆版本
	withoutMemory, err := e.RunScenario(ctx, scenario, false)
	if err != nil {
		return nil, fmt.Errorf("无记忆运行失败: %w", err)
	}

	// 运行有记忆版本
	withMemory, err := e.RunScenario(ctx, scenario, true)
	if err != nil {
		return nil, fmt.Errorf("有记忆运行失败: %w", err)
	}

	// 计算提升
	improvement := make(map[string]float64)

	// 正确率提升
	if withoutMemory.Metrics.CorrectnessScore > 0 {
		improvement["correctness"] = (withMemory.Metrics.CorrectnessScore - withoutMemory.Metrics.CorrectnessScore) /
			withoutMemory.Metrics.CorrectnessScore * 100
	}

	// 效率提升
	if withoutMemory.Metrics.EfficiencyScore > 0 {
		improvement["efficiency"] = (withMemory.Metrics.EfficiencyScore - withoutMemory.Metrics.EfficiencyScore) /
			withoutMemory.Metrics.EfficiencyScore * 100
	}

	// 综合得分提升
	if withoutMemory.Metrics.OverallScore > 0 {
		improvement["overall"] = (withMemory.Metrics.OverallScore - withoutMemory.Metrics.OverallScore) /
			withoutMemory.Metrics.OverallScore * 100
	}

	return &ComparisonResult{
		ScenarioID:    scenario.ID,
		WithMemory:    withMemory,
		WithoutMemory: withoutMemory,
		Improvement:   improvement,
		Significant:   improvement["overall"] > 10, // 简化判断：>10% 认为显著
	}, nil
}

// createAgentConfig 创建 Agent 配置
func (e *Executor) createAgentConfig(scenario *BenchmarkScenario, memoryEnabled bool) *config.Config {
	// 创建基础配置
	cfg := &config.Config{
		LLM: config.LLMConfig{
			Provider:    "qwen", // 默认使用 Qwen
			Model:       "qwen-plus",
			Temperature: scenario.Config.Temperature,
			MaxTokens:   4096,
		},
		Tool: config.ToolConfig{
			Timeout: 60,
		},
		WorkDir: ".",
	}

	// 覆盖场景配置
	if scenario.Config.MaxIterations > 0 {
		// 通过 session 配置传递
	}

	// 记忆配置
	cfg.Memory.Enabled = memoryEnabled
	if memoryEnabled {
		cfg.Memory.Dir = filepath.Join(os.Getenv("HOME"), ".oxencode", "benchmark_memory")
		cfg.Memory.BaseURL = e.config.MemoryServiceURL
	}

	return cfg
}

// createAgentComponents 创建 Agent 组件
func (e *Executor) createAgentComponents(ctx context.Context, cfg *config.Config) (*ctxpkg.Session, *ReActLoopForBenchmark, error) {
	// 创建 provider
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if apiKey == "" {
		return nil, nil, fmt.Errorf("DASHSCOPE_API_KEY not set")
	}

	provider, err := openaicompat.New(
		openaicompat.WithBaseURL("https://dashscope.aliyuncs.com/compatible-mode/v1"),
		openaicompat.WithAPIKey(apiKey),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("创建 provider 失败: %w", err)
	}
	e.provider = provider

	// 获取模型
	model, err := provider.LanguageModel(ctx, cfg.LLM.Model)
	if err != nil {
		return nil, nil, fmt.Errorf("获取模型失败: %w", err)
	}

	// 创建工具注册表
	log := logger.New("tools")
	registry := tools.NewRegistry(log)

	// 注册工具
	env, err := tools.NewLocalEnvironment(cfg.WorkDir, log)
	if err != nil {
		return nil, nil, fmt.Errorf("创建环境失败: %w", err)
	}

	registry.Register(tools.NewGlobTool(env, log))
	registry.Register(tools.NewGrepTool(env, log))
	registry.Register(tools.NewReadTool(env, log))
	registry.Register(tools.NewBashTool(env, log))
	registry.Register(tools.NewWriteTool(env, log))
	registry.Register(tools.NewEditTool(env, log))

	e.toolRegistry = registry

	// 创建 Session
	systemPrompt := "You are a helpful assistant."
	session, err := ctxpkg.NewSession(systemPrompt, cfg, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("创建 Session 失败: %w", err)
	}

	// 创建 ReActLoop
	reactLoop := NewReActLoopForBenchmark(&ReActLoopBenchmarkConfig{
		Model:    model,
		Session:  session,
		Registry: registry,
		Env:      env,
		Config:   cfg,
		Logger:   logger.New("react_bench"),
	})

	return session, reactLoop, nil
}

// executeSession 执行单个会话
func (e *Executor) executeSession(ctx context.Context, session *ctxpkg.Session, reactLoop *ReActLoopForBenchmark, spec *SessionSpec, sessionIndex int) (*SessionResult, error) {
	e.logger.Info("Executing session", "session_id", spec.ID, "index", sessionIndex)

	result := &SessionResult{
		SessionID: spec.ID,
	}

	startTime := time.Now()

	// 收集工具调用和响应
	var toolCalls []ToolCallRecord
	var responses []string
	iterations := 0

	// 设置回调收集数据
	reactLoop.SetCollectorCallbacks(&CollectorCallbacks{
		OnToolCall: func(record ToolCallRecord) {
			toolCalls = append(toolCalls, record)
		},
		OnResponse: func(content string) {
			responses = append(responses, content)
		},
		OnIteration: func() {
			iterations++
		},
	})

	// 执行每个消息
	for _, msg := range spec.Messages {
		if msg.Role == "user" {
			// 执行 ReAct 循环
			for event := range reactLoop.Stream(ctx, msg.Content) {
				switch event.Type {
				case "done":
					// 会话完成
				case "error":
					if event.Error != nil {
						result.Error = event.Error.Error()
					}
				}
			}
		}
	}

	result.Duration = time.Since(startTime)
	result.Iterations = iterations
	result.ToolCalls = toolCalls
	result.Responses = responses

	// 评估结果
	result.Scores = e.evaluateSession(result, &spec.Expected)
	result.ExpectationsMet = e.checkExpectations(result, &spec.Expected)

	return result, nil
}

// evaluateSession 评估会话结果
func (e *Executor) evaluateSession(result *SessionResult, expected *ExpectedOutcome) map[string]float64 {
	scores := make(map[string]float64)

	// 关键词匹配得分
	if len(expected.ResponseContains) > 0 {
		matched := 0
		for _, keyword := range expected.ResponseContains {
			for _, resp := range result.Responses {
				if strings.Contains(resp, keyword) {
					matched++
					break
				}
			}
		}
		scores["keyword_match"] = float64(matched) / float64(len(expected.ResponseContains))
	}

	// 工具正确性得分
	if len(expected.ToolCalls) > 0 {
		correctTools := 0
		for _, expectedTC := range expected.ToolCalls {
			for _, actualTC := range result.ToolCalls {
				if actualTC.Name == expectedTC.Name {
					// 检查输入匹配
					if e.checkInputMatch(actualTC.Input, expectedTC.InputContains) {
						correctTools++
						break
					}
				}
			}
		}
		scores["tool_correctness"] = float64(correctTools) / float64(len(expected.ToolCalls))
	}

	// 效率得分 (迭代次数)
	if len(result.Responses) > 0 {
		scores["efficiency"] = 1.0 / float64(max(result.Iterations, 1))
	}

	return scores
}

// checkInputMatch 检查输入匹配
func (e *Executor) checkInputMatch(actual map[string]any, expected map[string]any) bool {
	if len(expected) == 0 {
		return true
	}

	for k, v := range expected {
		if actualV, ok := actual[k]; ok {
			if fmt.Sprintf("%v", actualV) != fmt.Sprintf("%v", v) {
				return false
			}
		} else {
			return false
		}
	}
	return true
}

// checkExpectations 检查是否满足期望
func (e *Executor) checkExpectations(result *SessionResult, expected *ExpectedOutcome) bool {
	// 检查关键词
	for _, keyword := range expected.ResponseContains {
		found := false
		for _, resp := range result.Responses {
			if strings.Contains(resp, keyword) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查不应该包含的关键词
	for _, keyword := range expected.ResponseNotContain {
		for _, resp := range result.Responses {
			if strings.Contains(resp, keyword) {
				return false
			}
		}
	}

	return true
}

// calculateMetrics 计算聚合指标
func (e *Executor) calculateMetrics(sessions []SessionResult, scenario *BenchmarkScenario) Metrics {
	metrics := Metrics{}

	if len(sessions) == 0 {
		return metrics
	}

	// 平均正确性得分
	var totalCorrectness float64
	for _, s := range sessions {
		if score, ok := s.Scores["keyword_match"]; ok {
			totalCorrectness += score
		}
	}
	metrics.CorrectnessScore = totalCorrectness / float64(len(sessions))

	// 平均效率得分
	var totalEfficiency float64
	for _, s := range sessions {
		if score, ok := s.Scores["efficiency"]; ok {
			totalEfficiency += score
		}
	}
	metrics.EfficiencyScore = totalEfficiency / float64(len(sessions))

	// 综合得分
	weights := scenario.Evaluation.Weights
	if weights.Correctness == 0 && weights.Efficiency == 0 {
		weights = MetricWeights{
			Correctness: 0.5,
			Efficiency:  0.3,
			MemoryUsage: 0.2,
		}
	}
	metrics.OverallScore = metrics.CorrectnessScore*weights.Correctness +
		metrics.EfficiencyScore*weights.Efficiency

	return metrics
}

// CollectorCallbacks 数据收集回调
type CollectorCallbacks struct {
	OnToolCall  func(ToolCallRecord)
	OnResponse  func(string)
	OnIteration func()
}

// ReActLoopForBenchmark 基准测试用的 ReActLoop 封装
type ReActLoopForBenchmark struct {
	model    fantasy.LanguageModel
	session  *ctxpkg.Session
	registry *tools.Registry
	env      tools.Environment
	config   *config.Config
	logger   logger.Logger

	collector *CollectorCallbacks
}

// ReActLoopBenchmarkConfig 配置
type ReActLoopBenchmarkConfig struct {
	Model    fantasy.LanguageModel
	Session  *ctxpkg.Session
	Registry *tools.Registry
	Env      tools.Environment
	Config   *config.Config
	Logger   logger.Logger
}

// NewReActLoopForBenchmark 创建基准测试用 ReActLoop
func NewReActLoopForBenchmark(cfg *ReActLoopBenchmarkConfig) *ReActLoopForBenchmark {
	return &ReActLoopForBenchmark{
		model:    cfg.Model,
		session:  cfg.Session,
		registry: cfg.Registry,
		env:      cfg.Env,
		config:   cfg.Config,
		logger:   cfg.Logger,
	}
}

// SetCollectorCallbacks 设置收集回调
func (r *ReActLoopForBenchmark) SetCollectorCallbacks(cb *CollectorCallbacks) {
	r.collector = cb
}

// StreamEvent 流式事件
type StreamEvent struct {
	Type     string // "thought", "action", "observation", "content", "error", "done"
	Content  string
	ToolName string
	Error    error
}

// Stream 流式执行 (简化版，复用现有逻辑)
func (r *ReActLoopForBenchmark) Stream(ctx context.Context, userMessage string) <-chan StreamEvent {
	ch := make(chan StreamEvent, 100)

	go func() {
		defer close(ch)

		// 添加用户消息
		userMsg := message.NewMessage(message.RoleUser, userMessage)
		r.session.AddMessage(userMsg)

		// 这里应该调用实际的 LLM，但为了 benchmark 可以 mock
		// 简化实现：直接返回 done
		ch <- StreamEvent{Type: "done"}

		if r.collector != nil && r.collector.OnResponse != nil {
			r.collector.OnResponse("Response placeholder")
		}
	}()

	return ch
}

// 初始化 provider 工厂 (用于测试)
func createProviderForTest(cfg *config.Config) (fantasy.Provider, error) {
	apiKey := cfg.GetAPIKeyFromEnv()
	if apiKey == "" {
		return nil, fmt.Errorf("API key not found")
	}

	switch cfg.LLM.Provider {
	case config.ProviderAnthropic:
		return anthropic.New(anthropic.WithAPIKey(apiKey))
	case config.ProviderQwen:
		return openaicompat.New(
			openaicompat.WithBaseURL("https://dashscope.aliyuncs.com/compatible-mode/v1"),
			openaicompat.WithAPIKey(apiKey),
		)
	default:
		return openaicompat.New(openaicompat.WithAPIKey(apiKey))
	}
}