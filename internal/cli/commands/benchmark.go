package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yourname/oxencode/internal/benchmark"
	"github.com/yourname/oxencode/internal/cli/slashcmd"
)

const (
	// BenchmarkMemsvcPort benchmark 专用记忆服务端口
	BenchmarkMemsvcPort = 8766
	// BenchmarkMemoryDir benchmark 专用记忆目录
	BenchmarkMemoryDir = ".oxencode/benchmark_memory"
)

// BenchmarkCommand 基准测试命令
type BenchmarkCommand struct {
	runner        *benchmark.Runner
	memsvcProcess *exec.Cmd
}

// NewBenchmarkCommand 创建基准测试命令
func NewBenchmarkCommand() *BenchmarkCommand {
	return &BenchmarkCommand{
		runner: benchmark.NewRunner(&benchmark.BenchmarkConfig{
			MemoryServiceURL: fmt.Sprintf("http://127.0.0.1:%d", BenchmarkMemsvcPort),
			GenerateReport:   true,
			ReportFormat:     "markdown",
		}),
	}
}

// Name 返回命令名称
func (c *BenchmarkCommand) Name() string {
	return "benchmark"
}

// Description 返回命令描述
func (c *BenchmarkCommand) Description() string {
	return "运行记忆系统基准测试"
}

// Execute 执行命令
func (c *BenchmarkCommand) Execute(ctx context.Context, cmdCtx slashcmd.CommandContext, args string) (string, error) {
	// 解析参数
	opts, err := c.parseArgs(args)
	if err != nil {
		return "", err
	}

	// 列出场景
	if opts.list {
		return c.listScenarios()
	}

	// 启动专用记忆服务（如果需要）
	if opts.memory && opts.startService {
		if err := c.startBenchmarkMemsvc(); err != nil {
			return "", fmt.Errorf("启动记忆服务失败: %w", err)
		}
		defer c.stopBenchmarkMemsvc()
	}

	// 运行基准测试
	if opts.compare {
		return c.runComparison(ctx, opts)
	}

	return c.runBenchmark(ctx, opts)
}

// benchmarkOptions 命令选项
type benchmarkOptions struct {
	dimension    string
	scenarioIDs  []string
	compare      bool
	list         bool
	memory       bool
	startService bool
	outputDir    string
}

// parseArgs 解析命令参数
func (c *BenchmarkCommand) parseArgs(args string) (*benchmarkOptions, error) {
	opts := &benchmarkOptions{
		memory:       true,
		outputDir:    "benchmark_results",
		startService: false,
	}

	if args == "" {
		return opts, nil
	}

	parts := strings.Fields(args)
	for i := 0; i < len(parts); i++ {
		part := parts[i]

		switch part {
		case "--list", "-l":
			opts.list = true
		case "--compare", "-c":
			opts.compare = true
		case "--no-memory":
			opts.memory = false
		case "--start-service", "-S":
			opts.startService = true
		case "--dimension", "-d":
			if i+1 < len(parts) {
				opts.dimension = parts[i+1]
				i++
			} else {
				return nil, fmt.Errorf("--dimension 需要参数")
			}
		case "--output", "-o":
			if i+1 < len(parts) {
				opts.outputDir = parts[i+1]
				i++
			} else {
				return nil, fmt.Errorf("--output 需要参数")
			}
		case "--scenario", "-s":
			if i+1 < len(parts) {
				opts.scenarioIDs = append(opts.scenarioIDs, parts[i+1])
				i++
			} else {
				return nil, fmt.Errorf("--scenario 需要参数")
			}
		default:
			// 未知参数，可能是场景 ID
			if !strings.HasPrefix(part, "-") {
				opts.scenarioIDs = append(opts.scenarioIDs, part)
			}
		}
	}

	return opts, nil
}

// startBenchmarkMemsvc 启动专用的记忆服务
func (c *BenchmarkCommand) startBenchmarkMemsvc() error {
	// 检查是否已有服务运行
	checkCmd := exec.Command("curl", "-s", fmt.Sprintf("http://127.0.0.1:%d/api/v1/health", BenchmarkMemsvcPort))
	if err := checkCmd.Run(); err == nil {
		return nil // 服务已运行
	}

	// 设置环境变量
	homeDir, _ := os.UserHomeDir()
	memoryDir := filepath.Join(homeDir, BenchmarkMemoryDir)

	// 启动 memsvc
	cmd := exec.Command("uv", "run", "python", "-m", "memsvc", "--port", fmt.Sprintf("%d", BenchmarkMemsvcPort))
	cmd.Dir = "memsvc"
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("MEMSVC_MEMORY_DIR=%s", memoryDir),
		fmt.Sprintf("MEMSVC_DATA_DIR=%s", memoryDir),
		fmt.Sprintf("MEMSVC_PORT=%d", BenchmarkMemsvcPort),
	)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 memsvc 失败: %w", err)
	}

	c.memsvcProcess = cmd
	return nil
}

// stopBenchmarkMemsvc 停止专用的记忆服务
func (c *BenchmarkCommand) stopBenchmarkMemsvc() {
	if c.memsvcProcess != nil {
		c.memsvcProcess.Process.Kill()
		c.memsvcProcess = nil
	}
}

// listScenarios 列出场景
func (c *BenchmarkCommand) listScenarios() (string, error) {
	summaries, err := c.runner.ListScenarios()
	if err != nil {
		return "", fmt.Errorf("列出场景失败: %w", err)
	}

	var result strings.Builder
	result.WriteString("可用的基准测试场景:\n\n")

	// 按维度分组
	byDimension := make(map[string][]benchmark.ScenarioSummary)
	for _, s := range summaries {
		byDimension[string(s.Dimension)] = append(byDimension[string(s.Dimension)], s)
	}

	for dim, scenarios := range byDimension {
		result.WriteString(fmt.Sprintf("## %s\n", dim))
		for _, s := range scenarios {
			result.WriteString(fmt.Sprintf("  - %s: %s (%d sessions)\n", s.ID, s.Name, s.SessionCount))
		}
		result.WriteString("\n")
	}

	result.WriteString("\n使用方法:\n")
	result.WriteString("  /benchmark                    # 运行所有场景\n")
	result.WriteString("  /benchmark --list             # 列出所有场景\n")
	result.WriteString("  /benchmark --compare          # 对比模式\n")
	result.WriteString("  /benchmark --start-service    # 启动专用记忆服务\n")
	result.WriteString("  /benchmark -d tool_skill      # 运行特定维度\n")
	result.WriteString("  /benchmark <scenario_id>      # 运行特定场景\n")
	result.WriteString("\n记忆服务配置:\n")
	result.WriteString("  Benchmark 模式使用独立端口 8766 和隔离目录 ~/.oxencode/benchmark_memory\n")
	result.WriteString("  使用 --start-service 自动启动专用服务\n")

	return result.String(), nil
}

// runBenchmark 运行基准测试
func (c *BenchmarkCommand) runBenchmark(ctx context.Context, opts *benchmarkOptions) (string, error) {
	runOpts := &benchmark.RunOptions{
		MemoryEnabled: opts.memory,
		OutputDir:     opts.outputDir,
	}

	if opts.dimension != "" {
		runOpts.Dimension = benchmark.Dimension(opts.dimension)
	}
	runOpts.ScenarioIDs = opts.scenarioIDs

	results, err := c.runner.Run(ctx, runOpts)
	if err != nil {
		return "", fmt.Errorf("基准测试失败: %w", err)
	}

	// 打印结果
	c.runner.PrintResults(results)

	return fmt.Sprintf("基准测试完成，结果已保存到 %s/", opts.outputDir), nil
}

// runComparison 运行对比测试
func (c *BenchmarkCommand) runComparison(ctx context.Context, opts *benchmarkOptions) (string, error) {
	runOpts := &benchmark.RunOptions{
		CompareMode: true,
		OutputDir:   opts.outputDir,
	}

	if opts.dimension != "" {
		runOpts.Dimension = benchmark.Dimension(opts.dimension)
	}
	runOpts.ScenarioIDs = opts.scenarioIDs

	results, err := c.runner.RunComparison(ctx, runOpts)
	if err != nil {
		return "", fmt.Errorf("对比测试失败: %w", err)
	}

	// 打印结果
	c.runner.PrintComparisonResults(results)

	return fmt.Sprintf("对比测试完成，结果已保存到 %s/", opts.outputDir), nil
}

// Ensure BenchmarkCommand implements SlashCommand
var _ slashcmd.SlashCommand = (*BenchmarkCommand)(nil)