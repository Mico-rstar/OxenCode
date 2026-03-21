package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ScenarioLoader 场景加载器
type ScenarioLoader struct {
	scenariosDir string
}

// NewScenarioLoader 创建场景加载器
func NewScenarioLoader(scenariosDir string) *ScenarioLoader {
	return &ScenarioLoader{
		scenariosDir: scenariosDir,
	}
}

// LoadScenario 加载单个场景文件
func (l *ScenarioLoader) LoadScenario(path string) (*BenchmarkScenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取场景文件失败: %w", err)
	}

	var scenario BenchmarkScenario
	if err := json.Unmarshal(data, &scenario); err != nil {
		return nil, fmt.Errorf("解析场景文件失败: %w", err)
	}

	if err := l.validateScenario(&scenario); err != nil {
		return nil, fmt.Errorf("场景验证失败: %w", err)
	}

	return &scenario, nil
}

// LoadAllScenarios 加载所有场景
func (l *ScenarioLoader) LoadAllScenarios() ([]*BenchmarkScenario, error) {
	var scenarios []*BenchmarkScenario

	dimensions := []Dimension{
		DimensionToolSkill,
		DimensionLearning,
		DimensionFactRecall,
	}

	for _, dim := range dimensions {
		dimScenarios, err := l.LoadScenariosByDimension(dim)
		if err != nil {
			return nil, err
		}
		scenarios = append(scenarios, dimScenarios...)
	}

	return scenarios, nil
}

// LoadScenariosByDimension 按维度加载场景
func (l *ScenarioLoader) LoadScenariosByDimension(dim Dimension) ([]*BenchmarkScenario, error) {
	dimDir := filepath.Join(l.scenariosDir, string(dim))

	// 检查目录是否存在
	if _, err := os.Stat(dimDir); os.IsNotExist(err) {
		return nil, nil // 目录不存在，返回空列表
	}

	files, err := filepath.Glob(filepath.Join(dimDir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("扫描场景目录失败: %w", err)
	}

	var scenarios []*BenchmarkScenario
	for _, file := range files {
		scenario, err := l.LoadScenario(file)
		if err != nil {
			return nil, fmt.Errorf("加载场景 %s 失败: %w", file, err)
		}
		scenarios = append(scenarios, scenario)
	}

	return scenarios, nil
}

// LoadScenariosByIDs 按 ID 加载场景
func (l *ScenarioLoader) LoadScenariosByIDs(ids []string) ([]*BenchmarkScenario, error) {
	var scenarios []*BenchmarkScenario

	for _, id := range ids {
		scenario, err := l.findScenarioByID(id)
		if err != nil {
			return nil, err
		}
		scenarios = append(scenarios, scenario)
	}

	return scenarios, nil
}

// findScenarioByID 通过 ID 查找场景
func (l *ScenarioLoader) findScenarioByID(id string) (*BenchmarkScenario, error) {
	dimensions := []Dimension{
		DimensionToolSkill,
		DimensionLearning,
		DimensionFactRecall,
	}

	for _, dim := range dimensions {
		dimDir := filepath.Join(l.scenariosDir, string(dim))
		files, err := filepath.Glob(filepath.Join(dimDir, "*.json"))
		if err != nil {
			continue
		}

		for _, file := range files {
			scenario, err := l.LoadScenario(file)
			if err != nil {
				continue
			}
			if scenario.ID == id {
				return scenario, nil
			}
		}
	}

	return nil, fmt.Errorf("未找到场景: %s", id)
}

// validateScenario 验证场景定义
func (l *ScenarioLoader) validateScenario(s *BenchmarkScenario) error {
	if s.ID == "" {
		return fmt.Errorf("场景 ID 不能为空")
	}

	if s.Name == "" {
		return fmt.Errorf("场景名称不能为空")
	}

	// 验证维度
	validDimensions := map[Dimension]bool{
		DimensionToolSkill: true,
		DimensionLearning:  true,
		DimensionFactRecall: true,
	}
	if !validDimensions[s.Dimension] {
		return fmt.Errorf("无效的维度: %s", s.Dimension)
	}

	// 验证会话
	if len(s.Sessions) == 0 {
		return fmt.Errorf("场景必须至少包含一个会话")
	}

	for i, session := range s.Sessions {
		if err := l.validateSession(&session, i); err != nil {
			return err
		}
	}

	// 设置默认值
	l.setDefaults(s)

	return nil
}

// validateSession 验证会话定义
func (l *ScenarioLoader) validateSession(s *SessionSpec, index int) error {
	if s.ID == "" {
		return fmt.Errorf("会话 %d: ID 不能为空", index)
	}

	if len(s.Messages) == 0 {
		return fmt.Errorf("会话 %s: 必须至少包含一条消息", s.ID)
	}

	for i, msg := range s.Messages {
		if msg.Role == "" {
			return fmt.Errorf("会话 %s: 消息 %d 角色不能为空", s.ID, i)
		}
		if msg.Role != "user" && msg.Role != "assistant" {
			return fmt.Errorf("会话 %s: 消息 %d 角色必须是 'user' 或 'assistant'", s.ID, i)
		}
	}

	return nil
}

// setDefaults 设置默认值
func (l *ScenarioLoader) setDefaults(s *BenchmarkScenario) {
	if s.Config.MaxIterations == 0 {
		s.Config.MaxIterations = 10
	}
	if s.Config.TimeoutSeconds == 0 {
		s.Config.TimeoutSeconds = 120
	}
	if s.Config.Temperature == 0 {
		s.Config.Temperature = 0.7
	}
	if s.Version == "" {
		s.Version = "1.0"
	}

	// 设置评估权重默认值
	if s.Evaluation.Weights.Correctness == 0 &&
		s.Evaluation.Weights.Efficiency == 0 &&
		s.Evaluation.Weights.MemoryUsage == 0 {
		s.Evaluation.Weights = MetricWeights{
			Correctness:  0.5,
			Efficiency:   0.3,
			MemoryUsage:  0.2,
		}
	}
}

// SaveScenario 保存场景到文件
func (l *ScenarioLoader) SaveScenario(s *BenchmarkScenario) error {
	// 确定目标目录
	dimDir := filepath.Join(l.scenariosDir, string(s.Dimension))
	if err := os.MkdirAll(dimDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 生成文件名
	filename := strings.ReplaceAll(s.ID, "-", "_") + ".json"
	path := filepath.Join(dimDir, filename)

	// 序列化
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化场景失败: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入场景文件失败: %w", err)
	}

	return nil
}

// ScenarioSummary 场景摘要
type ScenarioSummary struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Dimension   Dimension `json:"dimension"`
	Tags        []string `json:"tags,omitempty"`
	SessionCount int     `json:"session_count"`
}

// ListScenarios 列出所有场景摘要
func (l *ScenarioLoader) ListScenarios() ([]ScenarioSummary, error) {
	scenarios, err := l.LoadAllScenarios()
	if err != nil {
		return nil, err
	}

	var summaries []ScenarioSummary
	for _, s := range scenarios {
		summaries = append(summaries, ScenarioSummary{
			ID:           s.ID,
			Name:         s.Name,
			Dimension:    s.Dimension,
			Tags:         s.Tags,
			SessionCount: len(s.Sessions),
		})
	}

	return summaries, nil
}