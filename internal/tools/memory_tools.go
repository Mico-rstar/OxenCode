package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yourname/oxencode/pkg/logger"
	"github.com/yourname/oxencode/pkg/memory"
)

// SearchMemoryTool RAG检索记忆工具
type SearchMemoryTool struct {
	client *memory.Client
	logger logger.Logger
}

// NewSearchMemoryTool 创建搜索记忆工具
func NewSearchMemoryTool(client *memory.Client, log logger.Logger) *SearchMemoryTool {
	if log == nil {
		log = logger.New("tool.search_memory")
	} else {
		log = log.Named("tool.search_memory")
	}
	return &SearchMemoryTool{
		client: client,
		logger: log,
	}
}

// Name 返回工具名称
func (t *SearchMemoryTool) Name() string {
	return "search_memory"
}

// Description 返回工具描述
func (t *SearchMemoryTool) Description() string {
	return `搜索记忆库中相关的内容。使用语义相似度检索，返回与查询最相关的记忆描述。

使用场景：
- 当需要查找相关的经验或知识时
- 当SystemReminder提示存在相关记忆时
- 当遇到可能需要历史上下文的问题时

返回：记忆描述列表，包含id、description和score`
}

// Parameters 返回参数schema
func (t *SearchMemoryTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"queries": {
				"type": "array",
				"items": {"type": "string"},
				"description": "搜索查询文本列表，支持多个查询"
			},
			"top_k": {
				"type": "integer",
				"description": "返回结果数量，默认5",
				"default": 5
			}
		},
		"required": ["queries"]
	}`)
}

// Validate 验证参数
func (t *SearchMemoryTool) Validate(input map[string]any) error {
	queries, ok := input["queries"]
	if !ok {
		return &ValidationError{Field: "queries", Message: "required field missing"}
	}

	queriesArr, ok := queries.([]any)
	if !ok {
		return &ValidationError{Field: "queries", Message: "must be an array"}
	}

	if len(queriesArr) == 0 {
		return &ValidationError{Field: "queries", Message: "must not be empty"}
	}

	return nil
}

// Execute 执行搜索
func (t *SearchMemoryTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	// 解析queries
	queriesRaw, ok := input["queries"].([]any)
	if !ok {
		return "", fmt.Errorf("invalid queries format")
	}
	queries := make([]string, len(queriesRaw))
	for i, q := range queriesRaw {
		queries[i], ok = q.(string)
		if !ok {
			return "", fmt.Errorf("query must be string")
		}
	}

	// 解析top_k
	topK := 5
	if topKRaw, ok := input["top_k"]; ok {
		if topKFloat, ok := topKRaw.(float64); ok {
			topK = int(topKFloat)
		}
	}

	// 调用记忆服务
	resp, err := t.client.SearchMemory(ctx, queries, topK)
	if err != nil {
		t.logger.Error("Search memory failed", "error", err)
		return "", err
	}

	// 格式化输出
	var result string
	if len(resp.Results) == 0 {
		result = "未找到相关记忆。"
	} else {
		result = fmt.Sprintf("找到 %d 条相关记忆：\n", len(resp.Results))
		for i, m := range resp.Results {
			result += fmt.Sprintf("\n%d. [%s] (相关度: %.2f)\n%s\n",
				i+1, m.ID, m.Score, m.Description)
		}
	}

	t.logger.Info("Search memory completed", "results", len(resp.Results))
	return result, nil
}

// LoadMemoryTool 加载完整记忆内容工具
type LoadMemoryTool struct {
	client *memory.Client
	logger logger.Logger
}

// NewLoadMemoryTool 创建加载记忆工具
func NewLoadMemoryTool(client *memory.Client, log logger.Logger) *LoadMemoryTool {
	if log == nil {
		log = logger.New("tool.load_memory")
	} else {
		log = log.Named("tool.load_memory")
	}
	return &LoadMemoryTool{
		client: client,
		logger: log,
	}
}

// Name 返回工具名称
func (t *LoadMemoryTool) Name() string {
	return "load_memory"
}

// Description 返回工具描述
func (t *LoadMemoryTool) Description() string {
	return `加载指定记忆的完整内容。

使用场景：
- 在search_memory后需要查看完整内容时
- 当需要深入了解某条记忆的详情时

参数：
- ids: 记忆ID列表（从search_memory获取）

返回：完整的记忆内容`
}

// Parameters 返回参数schema
func (t *LoadMemoryTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"ids": {
				"type": "array",
				"items": {"type": "string"},
				"description": "要加载的记忆ID列表"
			}
		},
		"required": ["ids"]
	}`)
}

// Validate 验证参数
func (t *LoadMemoryTool) Validate(input map[string]any) error {
	ids, ok := input["ids"]
	if !ok {
		return &ValidationError{Field: "ids", Message: "required field missing"}
	}

	idsArr, ok := ids.([]any)
	if !ok {
		return &ValidationError{Field: "ids", Message: "must be an array"}
	}

	if len(idsArr) == 0 {
		return &ValidationError{Field: "ids", Message: "must not be empty"}
	}

	return nil
}

// Execute 执行加载
func (t *LoadMemoryTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	// 解析ids
	idsRaw, ok := input["ids"].([]any)
	if !ok {
		return "", fmt.Errorf("invalid ids format")
	}
	ids := make([]string, len(idsRaw))
	for i, id := range idsRaw {
		ids[i], ok = id.(string)
		if !ok {
			return "", fmt.Errorf("id must be string")
		}
	}

	// 调用记忆服务
	resp, err := t.client.LoadMemory(ctx, ids)
	if err != nil {
		t.logger.Error("Load memory failed", "error", err)
		return "", err
	}

	// 格式化输出
	var result string
	if len(resp.Memories) == 0 {
		result = "未找到指定的记忆。"
	} else {
		result = fmt.Sprintf("加载 %d 条记忆：\n", len(resp.Memories))
		for _, m := range resp.Memories {
			result += fmt.Sprintf("\n--- %s ---\n来源: %s\n\n%s\n",
				m.ID, m.Source, m.Content)
		}
	}

	t.logger.Info("Load memory completed", "memories", len(resp.Memories))
	return result, nil
}