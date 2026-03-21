package benchmark

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// Evaluator 评估器接口
type Evaluator interface {
	// Name 返回评估器名称
	Name() string

	// Evaluate 评估会话结果
	Evaluate(ctx context.Context, result *SessionResult, expected *ExpectedOutcome) (float64, error)
}

// KeywordEvaluator 关键词匹配评估器
type KeywordEvaluator struct {
	caseSensitive bool
}

// NewKeywordEvaluator 创建关键词评估器
func NewKeywordEvaluator(caseSensitive bool) *KeywordEvaluator {
	return &KeywordEvaluator{
		caseSensitive: caseSensitive,
	}
}

// Name 返回名称
func (e *KeywordEvaluator) Name() string {
	return "keyword"
}

// Evaluate 评估
func (e *KeywordEvaluator) Evaluate(ctx context.Context, result *SessionResult, expected *ExpectedOutcome) (float64, error) {
	if len(expected.ResponseContains) == 0 {
		return 1.0, nil // 没有关键词要求，默认满分
	}

	// 合并所有响应
	allResponses := strings.Join(result.Responses, " ")
	if !e.caseSensitive {
		allResponses = strings.ToLower(allResponses)
	}

	matched := 0
	for _, keyword := range expected.ResponseContains {
		searchKeyword := keyword
		if !e.caseSensitive {
			searchKeyword = strings.ToLower(keyword)
		}
		if strings.Contains(allResponses, searchKeyword) {
			matched++
		}
	}

	return float64(matched) / float64(len(expected.ResponseContains)), nil
}

// RegexEvaluator 正则表达式评估器
type RegexEvaluator struct{}

// NewRegexEvaluator 创建正则评估器
func NewRegexEvaluator() *RegexEvaluator {
	return &RegexEvaluator{}
}

// Name 返回名称
func (e *RegexEvaluator) Name() string {
	return "regex"
}

// Evaluate 评估
func (e *RegexEvaluator) Evaluate(ctx context.Context, result *SessionResult, expected *ExpectedOutcome) (float64, error) {
	if len(expected.ResponseContains) == 0 {
		return 1.0, nil
	}

	allResponses := strings.Join(result.Responses, " ")

	matched := 0
	for _, pattern := range expected.ResponseContains {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue // 忽略无效正则
		}
		if re.MatchString(allResponses) {
			matched++
		}
	}

	return float64(matched) / float64(len(expected.ResponseContains)), nil
}

// ToolCallEvaluator 工具调用评估器
type ToolCallEvaluator struct{}

// NewToolCallEvaluator 创建工具调用评估器
func NewToolCallEvaluator() *ToolCallEvaluator {
	return &ToolCallEvaluator{}
}

// Name 返回名称
func (e *ToolCallEvaluator) Name() string {
	return "tool_call"
}

// Evaluate 评估工具调用
func (e *ToolCallEvaluator) Evaluate(ctx context.Context, result *SessionResult, expected *ExpectedOutcome) (float64, error) {
	if len(expected.ToolCalls) == 0 {
		return 1.0, nil
	}

	matched := 0
	for _, expectedTC := range expected.ToolCalls {
		for _, actualTC := range result.ToolCalls {
			if actualTC.Name == expectedTC.Name {
				// 检查输入匹配
				if e.checkInputMatch(actualTC.Input, expectedTC.InputContains) {
					// 检查不应该包含的输入
					if !e.checkInputNotMatch(actualTC.Input, expectedTC.InputNotContain) {
						matched++
						break
					}
				}
			}
		}
	}

	return float64(matched) / float64(len(expected.ToolCalls)), nil
}

// checkInputMatch 检查输入是否匹配期望
func (e *ToolCallEvaluator) checkInputMatch(actual map[string]any, expected map[string]any) bool {
	if len(expected) == 0 {
		return true
	}

	for k, v := range expected {
		actualV, ok := actual[k]
		if !ok {
			return false
		}
		if fmt.Sprintf("%v", actualV) != fmt.Sprintf("%v", v) {
			return false
		}
	}
	return true
}

// checkInputNotMatch 检查输入是否包含不应该有的内容
func (e *ToolCallEvaluator) checkInputNotMatch(actual map[string]any, expected map[string]any) bool {
	if len(expected) == 0 {
		return false
	}

	for k, v := range expected {
		actualV, ok := actual[k]
		if ok && fmt.Sprintf("%v", actualV) == fmt.Sprintf("%v", v) {
			return true // 匹配到了不应该有的内容
		}
	}
	return false
}

// CompositeEvaluator 组合评估器
type CompositeEvaluator struct {
	evaluators []Evaluator
	weights    []float64
}

// NewCompositeEvaluator 创建组合评估器
func NewCompositeEvaluator(evaluators []Evaluator, weights []float64) *CompositeEvaluator {
	if len(weights) == 0 {
		// 默认等权重
		weights = make([]float64, len(evaluators))
		for i := range weights {
			weights[i] = 1.0 / float64(len(evaluators))
		}
	}
	return &CompositeEvaluator{
		evaluators: evaluators,
		weights:    weights,
	}
}

// Name 返回名称
func (e *CompositeEvaluator) Name() string {
	return "composite"
}

// Evaluate 评估
func (e *CompositeEvaluator) Evaluate(ctx context.Context, result *SessionResult, expected *ExpectedOutcome) (float64, error) {
	if len(e.evaluators) == 0 {
		return 0, fmt.Errorf("没有配置评估器")
	}

	totalScore := 0.0
	totalWeight := 0.0

	for i, evaluator := range e.evaluators {
		score, err := evaluator.Evaluate(ctx, result, expected)
		if err != nil {
			continue // 忽略评估错误
		}

		weight := e.weights[i]
		totalScore += score * weight
		totalWeight += weight
	}

	if totalWeight > 0 {
		return totalScore / totalWeight, nil
	}
	return 0, nil
}

// EvaluationContext 评估上下文
type EvaluationContext struct {
	Evaluators map[string]Evaluator
}

// NewEvaluationContext 创建评估上下文
func NewEvaluationContext() *EvaluationContext {
	return &EvaluationContext{
		Evaluators: make(map[string]Evaluator),
	}
}

// RegisterEvaluator 注册评估器
func (c *EvaluationContext) RegisterEvaluator(name string, evaluator Evaluator) {
	c.Evaluators[name] = evaluator
}

// Evaluate 执行评估
func (c *EvaluationContext) Evaluate(ctx context.Context, result *SessionResult, expected *ExpectedOutcome, evaluatorNames []string) (map[string]float64, error) {
	scores := make(map[string]float64)

	for _, name := range evaluatorNames {
		if evaluator, ok := c.Evaluators[name]; ok {
			score, err := evaluator.Evaluate(ctx, result, expected)
			if err != nil {
				scores[name] = 0
			} else {
				scores[name] = score
			}
		}
	}

	return scores, nil
}

// DefaultEvaluationContext 创建默认评估上下文
func DefaultEvaluationContext() *EvaluationContext {
	ctx := NewEvaluationContext()
	ctx.RegisterEvaluator("keyword", NewKeywordEvaluator(false))
	ctx.RegisterEvaluator("keyword_case", NewKeywordEvaluator(true))
	ctx.RegisterEvaluator("regex", NewRegexEvaluator())
	ctx.RegisterEvaluator("tool_call", NewToolCallEvaluator())
	return ctx
}