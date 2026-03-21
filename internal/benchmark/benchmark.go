package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Runner 基准测试运行器
type Runner struct {
	config   *BenchmarkConfig
	executor *Executor
	loader   *ScenarioLoader
}

// NewRunner 创建运行器
func NewRunner(cfg *BenchmarkConfig) *Runner {
	return &Runner{
		config:   cfg,
		executor: NewExecutor(cfg),
		loader:   NewScenarioLoader("internal/benchmark/scenarios"),
	}
}

// RunOptions 运行选项
type RunOptions struct {
	Dimension     Dimension
	ScenarioIDs   []string
	CompareMode   bool
	MemoryEnabled bool
	OutputDir     string
}

// Run 执行基准测试
func (r *Runner) Run(ctx context.Context, opts *RunOptions) ([]*BenchmarkResult, error) {
	// 加载场景
	var scenarios []*BenchmarkScenario
	var err error

	if len(opts.ScenarioIDs) > 0 {
		scenarios, err = r.loader.LoadScenariosByIDs(opts.ScenarioIDs)
	} else if opts.Dimension != "" {
		scenarios, err = r.loader.LoadScenariosByDimension(opts.Dimension)
	} else {
		scenarios, err = r.loader.LoadAllScenarios()
	}

	if err != nil {
		return nil, fmt.Errorf("加载场景失败: %w", err)
	}

	if len(scenarios) == 0 {
		return nil, fmt.Errorf("没有找到可运行的场景")
	}

	// 执行场景
	var results []*BenchmarkResult
	for _, scenario := range scenarios {
		result, err := r.executor.RunScenario(ctx, scenario, opts.MemoryEnabled)
		if err != nil {
			result = &BenchmarkResult{
				ScenarioID: scenario.ID,
				Dimension:   scenario.Dimension,
				Timestamp:   time.Now(),
				Error:       err.Error(),
			}
		}
		results = append(results, result)
	}

	// 保存结果
	if opts.OutputDir != "" {
		if err := r.saveResults(results, opts.OutputDir); err != nil {
			return nil, fmt.Errorf("保存结果失败: %w", err)
		}
	}

	return results, nil
}

// RunComparison 执行对比测试
func (r *Runner) RunComparison(ctx context.Context, opts *RunOptions) ([]*ComparisonResult, error) {
	// 加载场景
	var scenarios []*BenchmarkScenario
	var err error

	if len(opts.ScenarioIDs) > 0 {
		scenarios, err = r.loader.LoadScenariosByIDs(opts.ScenarioIDs)
	} else if opts.Dimension != "" {
		scenarios, err = r.loader.LoadScenariosByDimension(opts.Dimension)
	} else {
		scenarios, err = r.loader.LoadAllScenarios()
	}

	if err != nil {
		return nil, fmt.Errorf("加载场景失败: %w", err)
	}

	var results []*ComparisonResult
	for _, scenario := range scenarios {
		result, err := r.executor.RunComparison(ctx, scenario)
		if err != nil {
			return nil, fmt.Errorf("场景 %s 对比测试失败: %w", scenario.ID, err)
		}
		results = append(results, result)
	}

	// 保存结果
	if opts.OutputDir != "" {
		if err := r.saveComparisonResults(results, opts.OutputDir); err != nil {
			return nil, fmt.Errorf("保存对比结果失败: %w", err)
		}
	}

	return results, nil
}

// ListScenarios 列出所有场景
func (r *Runner) ListScenarios() ([]ScenarioSummary, error) {
	return r.loader.ListScenarios()
}

// saveResults 保存结果
func (r *Runner) saveResults(results []*BenchmarkResult, outputDir string) error {
	// 创建输出目录
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	dir := filepath.Join(outputDir, timestamp)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 保存 JSON
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}

	jsonPath := filepath.Join(dir, "results.json")
	if err := os.WriteFile(jsonPath, data, 0644); err != nil {
		return err
	}

	// 生成 Markdown 报告
	report := r.generateReport(results)
	reportPath := filepath.Join(dir, "report.md")
	return os.WriteFile(reportPath, []byte(report), 0644)
}

// saveComparisonResults 保存对比结果
func (r *Runner) saveComparisonResults(results []*ComparisonResult, outputDir string) error {
	// 创建输出目录
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	dir := filepath.Join(outputDir, timestamp)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 保存 JSON
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}

	jsonPath := filepath.Join(dir, "comparison.json")
	if err := os.WriteFile(jsonPath, data, 0644); err != nil {
		return err
	}

	// 生成对比报告
	report := r.generateComparisonReport(results)
	reportPath := filepath.Join(dir, "comparison_report.md")
	return os.WriteFile(reportPath, []byte(report), 0644)
}

// generateReport 生成报告
func (r *Runner) generateReport(results []*BenchmarkResult) string {
	var report string

	report += "# Benchmark Results\n\n"
	report += fmt.Sprintf("Generated: %s\n\n", time.Now().Format(time.RFC3339))

	// 按维度分组
	byDimension := make(map[Dimension][]*BenchmarkResult)
	for _, result := range results {
		byDimension[result.Dimension] = append(byDimension[result.Dimension], result)
	}

	for dim, dimResults := range byDimension {
		report += fmt.Sprintf("## %s\n\n", dim)

		for _, result := range dimResults {
			report += fmt.Sprintf("### %s\n\n", result.ScenarioID)

			if result.Error != "" {
				report += fmt.Sprintf("**Error**: %s\n\n", result.Error)
				continue
			}

			report += fmt.Sprintf("- Duration: %v\n", result.Duration)
			report += fmt.Sprintf("- Memory Enabled: %v\n", result.MemoryEnabled)
			report += fmt.Sprintf("- Correctness Score: %.2f\n", result.Metrics.CorrectnessScore)
			report += fmt.Sprintf("- Efficiency Score: %.2f\n", result.Metrics.EfficiencyScore)
			report += fmt.Sprintf("- Overall Score: %.2f\n\n", result.Metrics.OverallScore)

			// 会话详情
			for i, session := range result.Sessions {
				report += fmt.Sprintf("#### Session %d: %s\n\n", i+1, session.SessionID)
				report += fmt.Sprintf("- Iterations: %d\n", session.Iterations)
				report += fmt.Sprintf("- Tool Calls: %d\n", len(session.ToolCalls))
				report += fmt.Sprintf("- Expectations Met: %v\n\n", session.ExpectationsMet)
			}
		}
	}

	return report
}

// generateComparisonReport 生成对比报告
func (r *Runner) generateComparisonReport(results []*ComparisonResult) string {
	var report string

	report += "# Benchmark Comparison Report\n\n"
	report += fmt.Sprintf("Generated: %s\n\n", time.Now().Format(time.RFC3339))

	report += "## Summary\n\n"
	report += "| Scenario | Without Memory | With Memory | Improvement | Significant |\n"
	report += "|----------|---------------|-------------|-------------|-------------|\n"

	for _, result := range results {
		withoutScore := 0.0
		withScore := 0.0
		if result.WithoutMemory != nil {
			withoutScore = result.WithoutMemory.Metrics.OverallScore
		}
		if result.WithMemory != nil {
			withScore = result.WithMemory.Metrics.OverallScore
		}

		improvement := "N/A"
		if imp, ok := result.Improvement["overall"]; ok {
			improvement = fmt.Sprintf("%.1f%%", imp)
		}

		significant := "No"
		if result.Significant {
			significant = "Yes"
		}

		report += fmt.Sprintf("| %s | %.2f | %.2f | %s | %s |\n",
			result.ScenarioID, withoutScore, withScore, improvement, significant)
	}

	report += "\n## Details\n\n"

	for _, result := range results {
		report += fmt.Sprintf("### %s\n\n", result.ScenarioID)

		if result.WithoutMemory != nil {
			report += "**Without Memory:**\n"
			report += fmt.Sprintf("- Correctness: %.2f\n", result.WithoutMemory.Metrics.CorrectnessScore)
			report += fmt.Sprintf("- Efficiency: %.2f\n", result.WithoutMemory.Metrics.EfficiencyScore)
			report += fmt.Sprintf("- Overall: %.2f\n\n", result.WithoutMemory.Metrics.OverallScore)
		}

		if result.WithMemory != nil {
			report += "**With Memory:**\n"
			report += fmt.Sprintf("- Correctness: %.2f\n", result.WithMemory.Metrics.CorrectnessScore)
			report += fmt.Sprintf("- Efficiency: %.2f\n", result.WithMemory.Metrics.EfficiencyScore)
			report += fmt.Sprintf("- Overall: %.2f\n\n", result.WithMemory.Metrics.OverallScore)
		}

		report += "**Improvements:**\n"
		for metric, value := range result.Improvement {
			report += fmt.Sprintf("- %s: %.1f%%\n", metric, value)
		}
		report += "\n"
	}

	return report
}

// PrintResults 打印结果摘要
func (r *Runner) PrintResults(results []*BenchmarkResult) {
	fmt.Println("\n=== Benchmark Results ===")
	fmt.Println()

	for _, result := range results {
		fmt.Printf("Scenario: %s\n", result.ScenarioID)
		fmt.Printf("  Dimension: %s\n", result.Dimension)

		if result.Error != "" {
			fmt.Printf("  Error: %s\n", result.Error)
		} else {
			fmt.Printf("  Duration: %v\n", result.Duration)
			fmt.Printf("  Overall Score: %.2f\n", result.Metrics.OverallScore)
			fmt.Printf("  Sessions: %d\n", len(result.Sessions))
		}
		fmt.Println()
	}
}

// PrintComparisonResults 打印对比结果
func (r *Runner) PrintComparisonResults(results []*ComparisonResult) {
	fmt.Println("\n=== Benchmark Comparison Results ===")
	fmt.Println()

	for _, result := range results {
		fmt.Printf("Scenario: %s\n", result.ScenarioID)

		if result.WithoutMemory != nil {
			fmt.Printf("  Without Memory: %.2f\n", result.WithoutMemory.Metrics.OverallScore)
		}

		if result.WithMemory != nil {
			fmt.Printf("  With Memory:    %.2f\n", result.WithMemory.Metrics.OverallScore)
		}

		if imp, ok := result.Improvement["overall"]; ok {
			significant := ""
			if result.Significant {
				significant = " (*)"
			}
			fmt.Printf("  Improvement:    %.1f%%%s\n", imp, significant)
		}
		fmt.Println()
	}
}