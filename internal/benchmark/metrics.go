package benchmark

import (
	"math"
	"time"
)

// MetricsCollector 指标收集器
type MetricsCollector struct {
	sessionID string
	startTime time.Time

	toolCalls  []ToolCallRecord
	memoryOps  []MemoryOpRecord
	responses  []string
	iterations int
	errors     []error
}

// NewMetricsCollector 创建指标收集器
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		toolCalls: make([]ToolCallRecord, 0),
		memoryOps: make([]MemoryOpRecord, 0),
		responses: make([]string, 0),
		errors:    make([]error, 0),
	}
}

// Start 开始收集
func (c *MetricsCollector) Start(sessionID string) {
	c.sessionID = sessionID
	c.startTime = time.Now()
	c.toolCalls = make([]ToolCallRecord, 0)
	c.memoryOps = make([]MemoryOpRecord, 0)
	c.responses = make([]string, 0)
	c.iterations = 0
	c.errors = make([]error, 0)
}

// RecordToolCall 记录工具调用
func (c *MetricsCollector) RecordToolCall(name string, input map[string]any, output string, err error) {
	record := ToolCallRecord{
		Iteration: c.iterations,
		Name:      name,
		Input:     input,
		Output:    output,
		Timestamp: time.Now(),
	}
	if err != nil {
		record.Error = err.Error()
	}
	c.toolCalls = append(c.toolCalls, record)
}

// RecordMemoryOp 记录记忆操作
func (c *MetricsCollector) RecordMemoryOp(operation, query string, results []string) {
	c.memoryOps = append(c.memoryOps, MemoryOpRecord{
		Operation: operation,
		Query:     query,
		Results:   results,
		Timestamp: time.Now(),
	})
}

// RecordResponse 记录响应
func (c *MetricsCollector) RecordResponse(content string) {
	c.responses = append(c.responses, content)
}

// IncrementIteration 增加迭代次数
func (c *MetricsCollector) IncrementIteration() {
	c.iterations++
}

// RecordError 记录错误
func (c *MetricsCollector) RecordError(err error) {
	c.errors = append(c.errors, err)
}

// Finish 完成收集，返回会话结果
func (c *MetricsCollector) Finish() *SessionResult {
	return &SessionResult{
		SessionID:  c.sessionID,
		Duration:   time.Since(c.startTime),
		Iterations: c.iterations + 1,
		ToolCalls:  c.toolCalls,
		MemoryOps:  c.memoryOps,
		Responses:  c.responses,
	}
}

// MetricsCalculator 指标计算器
type MetricsCalculator struct{}

// NewMetricsCalculator 创建指标计算器
func NewMetricsCalculator() *MetricsCalculator {
	return &MetricsCalculator{}
}

// CalculateToolSkillMetrics 计算工具技能学习指标
func (c *MetricsCalculator) CalculateToolSkillMetrics(sessions []SessionResult) *LearningMetrics {
	if len(sessions) < 2 {
		return nil
	}

	first := sessions[0]
	later := sessions[1:]

	// 统计第一个会话的错误
	mistakesFirst := c.countToolErrors(&first)
	iterFirst := first.Iterations

	// 统计后续会话的错误
	mistakesLater := 0
	iterLater := 0
	for _, s := range later {
		mistakesLater += c.countToolErrors(&s)
		iterLater += s.Iterations
	}
	if len(later) > 0 {
		iterLater /= len(later)
	}

	// 计算避免率
	mistakeAvoidanceRate := 0.0
	if mistakesFirst > 0 {
		mistakeAvoidanceRate = float64(mistakesFirst-mistakesLater) / float64(mistakesFirst) * 100
	}

	// 计算效率提升
	efficiencyImprovement := 0.0
	if iterFirst > 0 {
		efficiencyImprovement = float64(iterFirst-iterLater) / float64(iterFirst) * 100
	}

	return &LearningMetrics{
		MistakesInFirstSession:   mistakesFirst,
		MistakesInLaterSession:   mistakesLater,
		MistakeAvoidanceRate:     mistakeAvoidanceRate,
		IterationsInFirstSession: iterFirst,
		IterationsInLaterSession: iterLater,
		EfficiencyImprovement:    efficiencyImprovement,
	}
}

// CalculateRecallMetrics 计算事实召回指标
func (c *MetricsCalculator) CalculateRecallMetrics(sessions []SessionResult, factsInjected int) *RecallMetrics {
	if len(sessions) == 0 || factsInjected == 0 {
		return nil
	}

	totalRecalled := 0
	correctRecalls := 0
	totalRetrievalTime := 0.0

	for _, s := range sessions {
		// 统计记忆操作
		for _, op := range s.MemoryOps {
			if op.Operation == "search" || op.Operation == "load" {
				totalRecalled += len(op.Results)

				// 计算检索时间（如果有）
				// 这里简化处理，实际应该记录精确时间
			}
		}

		// 根据响应判断正确召回
		// 这里需要更复杂的逻辑来判断
		_ = s.Responses // 暂时忽略
	}

	precision := 0.0
	if totalRecalled > 0 {
		precision = float64(correctRecalls) / float64(totalRecalled)
	}

	recallRate := float64(totalRecalled) / float64(factsInjected)

	avgRetrievalTime := 0.0
	if totalRecalled > 0 {
		avgRetrievalTime = totalRetrievalTime / float64(totalRecalled)
	}

	return &RecallMetrics{
		FactsInjected:      factsInjected,
		FactsRecalled:      totalRecalled,
		RecallRate:         recallRate,
		CorrectRecalls:     correctRecalls,
		IncorrectRecalls:   totalRecalled - correctRecalls,
		Precision:          precision,
		AvgRetrievalTimeMs: avgRetrievalTime,
	}
}

// countToolErrors 统计工具调用错误次数
func (c *MetricsCalculator) countToolErrors(session *SessionResult) int {
	count := 0
	for _, tc := range session.ToolCalls {
		if tc.Error != "" {
			count++
		}
	}
	return count
}

// CalculateOverallScore 计算综合得分
func (c *MetricsCalculator) CalculateOverallScore(metrics *Metrics, weights MetricWeights) float64 {
	score := 0.0
	totalWeight := 0.0

	if weights.Correctness > 0 {
		score += metrics.CorrectnessScore * weights.Correctness
		totalWeight += weights.Correctness
	}
	if weights.Efficiency > 0 {
		score += metrics.EfficiencyScore * weights.Efficiency
		totalWeight += weights.Efficiency
	}
	if weights.MemoryUsage > 0 {
		score += metrics.MemoryUsageScore * weights.MemoryUsage
		totalWeight += weights.MemoryUsage
	}
	if weights.LearningRate > 0 && metrics.LearningMetrics != nil {
		learningScore := metrics.LearningMetrics.MistakeAvoidanceRate / 100
		score += learningScore * weights.LearningRate
		totalWeight += weights.LearningRate
	}

	if totalWeight > 0 {
		return score / totalWeight
	}
	return 0
}

// CalculateStatistics 计算统计信息
func (c *MetricsCalculator) CalculateStatistics(results []BenchmarkResult) map[string]StatisticalSummary {
	stats := make(map[string]StatisticalSummary)

	// 收集各指标值
	correctnessScores := make([]float64, 0)
	efficiencyScores := make([]float64, 0)
	overallScores := make([]float64, 0)

	for _, r := range results {
		correctnessScores = append(correctnessScores, r.Metrics.CorrectnessScore)
		efficiencyScores = append(efficiencyScores, r.Metrics.EfficiencyScore)
		overallScores = append(overallScores, r.Metrics.OverallScore)
	}

	stats["correctness"] = c.calculateStats(correctnessScores)
	stats["efficiency"] = c.calculateStats(efficiencyScores)
	stats["overall"] = c.calculateStats(overallScores)

	return stats
}

// StatisticalSummary 统计摘要
type StatisticalSummary struct {
	Mean   float64 `json:"mean"`
	StdDev float64 `json:"std_dev"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
}

// calculateStats 计算统计摘要
func (c *MetricsCalculator) calculateStats(values []float64) StatisticalSummary {
	if len(values) == 0 {
		return StatisticalSummary{}
	}

	// 计算均值
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	// 计算标准差
	variance := 0.0
	for _, v := range values {
		variance += (v - mean) * (v - mean)
	}
	stdDev := math.Sqrt(variance / float64(len(values)))

	// 计算最小最大值
	min := values[0]
	max := values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	return StatisticalSummary{
		Mean:   mean,
		StdDev: stdDev,
		Min:    min,
		Max:    max,
	}
}