// Package benchmark 提供记忆系统的基准测试框架
package benchmark

import (
	"time"
)

// Dimension 基准测试维度
type Dimension string

const (
	DimensionToolSkill Dimension = "tool_skill"  // 工具技能学习
	DimensionLearning  Dimension = "learning"    // 学习能力
	DimensionFactRecall Dimension = "fact_recall" // 事实召回
)

// BenchmarkScenario 基准测试场景定义
type BenchmarkScenario struct {
	// 元数据
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Dimension   Dimension `json:"dimension"`
	Tags        []string `json:"tags,omitempty"`
	Version     string   `json:"version"`

	// 配置
	Config ScenarioConfig `json:"config"`

	// 记忆预置条件
	MemoryPrecondition MemoryPrecondition `json:"memory_precondition"`

	// 会话列表
	Sessions []SessionSpec `json:"sessions"`

	// 评估规格
	Evaluation EvaluationSpec `json:"evaluation"`
}

// ScenarioConfig 场景配置
type ScenarioConfig struct {
	MaxIterations  int     `json:"max_iterations,omitempty"`  // 最大迭代次数
	TimeoutSeconds int     `json:"timeout_seconds,omitempty"` // 超时时间
	Temperature    float64 `json:"temperature,omitempty"`    // LLM 温度
	MemoryEnabled  bool    `json:"memory_enabled"`           // 是否启用记忆
}

// MemoryPrecondition 记忆预置条件
type MemoryPrecondition struct {
	Experience []MemoryEntry `json:"experience,omitempty"`
	Knowledge  []MemoryEntry `json:"knowledge,omitempty"`
	Notes      []MemoryEntry `json:"notes,omitempty"`
	InnerSelf  string        `json:"inner_self,omitempty"`
	InnerUser  string        `json:"inner_user,omitempty"`
}

// MemoryEntry 记忆条目
type MemoryEntry struct {
	ID          string            `json:"id"`
	Description string            `json:"description"` // 用于 RAG 检索
	Content     string            `json:"content"`     // 完整内容
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// SessionSpec 会话规格
type SessionSpec struct {
	ID          string          `json:"id"`
	Description string          `json:"description,omitempty"`
	Messages    []MessageSpec   `json:"messages"`
	Expected    ExpectedOutcome `json:"expected,omitempty"`
	WaitAfter   int             `json:"wait_after_seconds,omitempty"` // 等待后处理时间
}

// MessageSpec 消息规格
type MessageSpec struct {
	Role    string `json:"role"`    // "user" 或 "assistant"
	Content string `json:"content"`
}

// ExpectedOutcome 预期结果
type ExpectedOutcome struct {
	// 工具调用期望
	ToolCalls []ExpectedToolCall `json:"tool_calls,omitempty"`

	// 响应期望
	ResponseContains   []string `json:"response_contains,omitempty"`
	ResponseNotContain []string `json:"response_not_contain,omitempty"`

	// 行为期望
	ShouldAvoidMistake bool `json:"should_avoid_mistake"`
	ShouldUseMemory    bool `json:"should_use_memory"`

	// 指标阈值
	MaxIterations  int     `json:"max_iterations,omitempty"`
	MinCorrectness float64 `json:"min_correctness,omitempty"`
}

// ExpectedToolCall 预期工具调用
type ExpectedToolCall struct {
	Name           string         `json:"name"`
	InputContains  map[string]any `json:"input_contains,omitempty"`
	InputNotContain map[string]any `json:"input_not_contain,omitempty"`
}

// EvaluationSpec 评估规格
type EvaluationSpec struct {
	Weights    MetricWeights   `json:"weights"`
	Evaluators []EvaluatorSpec `json:"evaluators,omitempty"`
}

// MetricWeights 指标权重
type MetricWeights struct {
	Correctness  float64 `json:"correctness,omitempty"`
	Efficiency   float64 `json:"efficiency,omitempty"`
	MemoryUsage  float64 `json:"memory_usage,omitempty"`
	LearningRate float64 `json:"learning_rate,omitempty"`
}

// EvaluatorSpec 评估器规格
type EvaluatorSpec struct {
	Name   string         `json:"name"`
	Type   string         `json:"type"` // "keyword", "llm_judge", "semantic"
	Config map[string]any `json:"config,omitempty"`
}

// ============================================
// 结果类型
// ============================================

// BenchmarkResult 基准测试结果
type BenchmarkResult struct {
	ScenarioID string    `json:"scenario_id"`
	Dimension  Dimension `json:"dimension"`
	Timestamp  time.Time `json:"timestamp"`
	Duration   time.Duration `json:"duration"`

	// 记忆配置
	MemoryEnabled bool `json:"memory_enabled"`

	// 会话结果
	Sessions []SessionResult `json:"sessions"`

	// 聚合指标
	Metrics Metrics `json:"metrics"`

	// 错误信息
	Error string `json:"error,omitempty"`
}

// SessionResult 会话结果
type SessionResult struct {
	SessionID  string        `json:"session_id"`
	Duration   time.Duration `json:"duration"`
	Iterations int           `json:"iterations"`

	// 工具调用记录
	ToolCalls []ToolCallRecord `json:"tool_calls,omitempty"`

	// 记忆操作记录
	MemoryOps []MemoryOpRecord `json:"memory_ops,omitempty"`

	// LLM 响应
	Responses []string `json:"responses,omitempty"`

	// 评估得分
	Scores map[string]float64 `json:"scores,omitempty"`

	// 是否满足期望
	ExpectationsMet bool `json:"expectations_met"`

	// 错误信息
	Error string `json:"error,omitempty"`
}

// ToolCallRecord 工具调用记录
type ToolCallRecord struct {
	Iteration int            `json:"iteration"`
	Name      string         `json:"name"`
	Input     map[string]any `json:"input,omitempty"`
	Output    string         `json:"output,omitempty"`
	Error     string         `json:"error,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// MemoryOpRecord 记忆操作记录
type MemoryOpRecord struct {
	Operation string    `json:"operation"` // "trigger", "search", "load"
	Query     string    `json:"query,omitempty"`
	Results   []string  `json:"results,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Metrics 聚合指标
type Metrics struct {
	// 主要指标
	CorrectnessScore float64 `json:"correctness_score"`
	EfficiencyScore  float64 `json:"efficiency_score"`
	MemoryUsageScore float64 `json:"memory_usage_score"`

	// 维度特定指标
	LearningMetrics *LearningMetrics `json:"learning_metrics,omitempty"`
	RecallMetrics   *RecallMetrics   `json:"recall_metrics,omitempty"`

	// 综合得分
	OverallScore float64 `json:"overall_score"`
}

// LearningMetrics 学习指标
type LearningMetrics struct {
	MistakesInFirstSession  int     `json:"mistakes_in_first_session"`
	MistakesInLaterSession  int     `json:"mistakes_in_later_session"`
	MistakeAvoidanceRate    float64 `json:"mistake_avoidance_rate"`

	IterationsInFirstSession int     `json:"iterations_in_first_session"`
	IterationsInLaterSession int     `json:"iterations_in_later_session"`
	EfficiencyImprovement    float64 `json:"efficiency_improvement"`

	MemoryTriggered bool `json:"memory_triggered"`
	MemoryUsedForCorrection bool `json:"memory_used_for_correction"`
}

// RecallMetrics 召回指标
type RecallMetrics struct {
	FactsInjected     int     `json:"facts_injected"`
	FactsRecalled     int     `json:"facts_recalled"`
	RecallRate        float64 `json:"recall_rate"`

	CorrectRecalls    int     `json:"correct_recalls"`
	IncorrectRecalls  int     `json:"incorrect_recalls"`
	Precision         float64 `json:"precision"`

	AvgRetrievalTimeMs float64 `json:"avg_retrieval_time_ms"`
}

// ComparisonResult 对比结果
type ComparisonResult struct {
	ScenarioID     string             `json:"scenario_id"`
	WithMemory     *BenchmarkResult   `json:"with_memory"`
	WithoutMemory  *BenchmarkResult   `json:"without_memory"`
	Improvement    map[string]float64 `json:"improvement"`
	Significant    bool               `json:"significant"` // 统计显著性
}

// ============================================
// 配置类型
// ============================================

// BenchmarkConfig 基准测试配置
type BenchmarkConfig struct {
	// 记忆服务配置
	MemoryServiceURL string
	MemoryEnabled    bool

	// LLM 配置
	Provider    string
	Model       string
	Temperature float64
	MaxTokens   int

	// 基准测试设置
	ParallelScenarios int
	TimeoutSeconds    int
	OutputDir         string

	// 报告设置
	GenerateReport bool
	ReportFormat   string // "json", "markdown"
}

// MemoryState 记忆状态快照
type MemoryState struct {
	ExperienceCount int               `json:"experience_count"`
	KnowledgeCount  int               `json:"knowledge_count"`
	NotesCount      int               `json:"notes_count"`
	InnerSelf       string            `json:"inner_self,omitempty"`
	InnerUser       string            `json:"inner_user,omitempty"`
	VectorCount     int               `json:"vector_count"`
	Files           map[string]string `json:"files,omitempty"`
}