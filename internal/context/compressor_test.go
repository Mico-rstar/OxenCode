package context

import (
	"context"
	"fmt"
	"testing"
	"time"

	openaicompat "charm.land/fantasy/providers/openaicompat"
	"github.com/yourname/oxencode/internal/message"
	"github.com/yourname/oxencode/pkg/config"
	"github.com/yourname/oxencode/pkg/logger"
	"github.com/yourname/oxencode/pkg/prompt"
)

// TestL0CompressWithRealLLM 测试L0级别压缩使用真实LLM
func TestL0CompressWithRealLLM(t *testing.T) {
	ctx := context.Background()

	// 创建 provider（使用通义千问 DashScope API）
	provider, err := openaicompat.New(
		openaicompat.WithBaseURL("https://dashscope.aliyuncs.com/compatible-mode/v1"),
		openaicompat.WithAPIKey("sk-8440a58764c846c88183bfec2d94d279"),
	)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// 加载提示词
	p := prompt.New("../../pkg/prompt/prompts")
	if err := p.Load(); err != nil {
		t.Fatalf("Failed to load prompts: %v", err)
	}

	// 创建L0压缩策略
	strategy := &CompressionStrategy{
		Skill:            p.L0Skill,
		CompressionModel: "qwen-plus",
		Timeout:          60 * time.Second,
	}

	// 创建配置
	cfg := &config.Config{
		Compressor: config.CompressorConfig{
			Model:       "qwen-plus",
			MaxTokens:   4096,
			Temperature: 0.3,
		},
	}

	// 创建压缩器
	compressor, err := NewLLMCompressor(ctx, provider, strategy, cfg, logger.NewNop(), p)
	if err != nil {
		t.Fatalf("Failed to create compressor: %v", err)
	}

	// 准备测试数据（模拟多轮对话历史）
	rawContent := "[User]\nalterdo后端系统架构是什么样的\n\n[Tool Result]\n./alterdo/backend/internal/model/user.go\n./alterdo/backend/internal/model/ai_request.go\n./alterdo/backend/internal/model/skillhub.go\n./alterdo/backend/internal/model/token_usage.go\n./alterdo/backend/internal/model/usage.go\n./alterdo/backend/internal/middleware/cors.go\n./alterdo/backend/internal/middleware/logger.go\n./alterdo/backend/internal/middleware/apikey_auth.go\n./alterdo/backend/internal/middleware/admin.go\n./alterdo/backend/internal/middleware/recovery.go\n./alterdo/backend/internal/middleware/auth.go\n./alterdo/backend/internal/middleware/request_id.go\n./alterdo/backend/internal/service/balance.go\n./alterdo/backend/internal/service/token_collector.go\n./alterdo/backend/internal/service/rate_limiter.go\n./alterdo/backend/internal/service/session.go\n./alterdo/backend/internal/service/auth.go\n./alterdo/backend/internal/service/ai_proxy.go\n./alterdo/backend/internal/service/usage.go\n./alterdo/backend/internal/handler/user.go\n./alterdo/backend/internal/handler/skillhub.go\n./alterdo/back\n[...truncated]\n\n[Tool Result]\n    1→// Package config provides configuration management for the application.\n    2→// Configuration is loaded from multiple sources with the following priority (highest to lowest):\n    3→// 1. Environment variables (only if explicitly set)\n    4→// 2. config.toml file\n    5→// 3. Default values\n    6→package config\n    7→\n    8→import (\n    9→\t\"fmt\"\n   10→\t\"time\"\n   11→)\n   12→\n   13→// Config represents the application configuration.\n   14→type Config struct {\n   15→\tServer        ServerConfig        `mapstructure:\"server\"`\n   16→\tDatabase      DatabaseConfig      `mapstructure:\"database\"`\n   17→\tRedis         RedisConfig         `mapstructure:\"redis\"`\n   18→\tJWT           JWTConfig           `mapstructure:\"jwt\"`\n   19→\tLog           LogConfig           `mapstructure:\"log\"`\n   20→\tCORS          CORSConfig          `mapstructure:\"cors\"`\n   21→\tSessionServer SessionServerConfig `mapstructure:\"session_server\"`\n   22→\tNewAPI        NewAPIConfi\n[...truncated]\n\n[Tool Result]\n    1→// Package handler provides HTTP request handlers.\n    2→package handler\n    3→\n    4→import (\n    5→\t\"net/http\"\n    6→\n    7→\t\"github.com/gin-gonic/gin\"\n    8→\n    9→\t\"github.com/alterdo/backend/internal/service\"\n   10→\t\"github.com/alterdo/backend/pkg/logger\"\n   11→\t\"github.com/alterdo/backend/pkg/response\"\n   12→)\n   13→\n   14→// AIProxyHandler handles AI proxy requests.\n   15→type AIProxyHandler struct {\n   16→\taiProxySvc *service.AIProxyService\n   17→\tlogger     logger.Logger\n   18→}\n   19→\n   20→// NewAIProxyHandler creates a new AIProxyHandler instance.\n   21→func NewAIProxyHandler(aiProxySvc *service.AIProxyService, log logger.Logger) *AIProxyHandler {\n   22→\treturn \u0026AIProxyHandler{\n   23→\t\taiProxySvc: aiProxySvc,\n   24→\t\tlogger:     log,\n   25→\t}\n   26→}\n   27→\n   28→// Messages handles POST /v1/messages - Claude API compatible endpoint.\n   29→// This is a pure HTTP reverse proxy that forwards requests to the confi\n[...truncated]\n\n[Tool Result]\n    1→// Package service provides business logic services for the application.\n    2→package service\n    3→\n    4→import (\n    5→\t\"bufio\"\n    6→\t\"bytes\"\n    7→\t\"context\"\n    8→\t\"encoding/json\"\n    9→\t\"fmt\"\n   10→\t\"io\"\n   11→\t\"net\"\n   12→\t\"net/http\"\n   13→\t\"net/http/httputil\"\n   14→\t\"net/url\"\n   15→\t\"os\"\n   16→\t\"path/filepath\"\n   17→\t\"strconv\"\n   18→\t\"strings\"\n   19→\t\"time\"\n   20→\n   21→\t\"github.com/alterdo/backend/internal/model\"\n   22→\t\"github.com/alterdo/backend/internal/store\"\n   23→\t\"github.com/alterdo/backend/pkg/config\"\n   24→\t\"github.com/alterdo/backend/pkg/logger\"\n   25→)\n   26→\n   27→// AIProxyService handles AI request proxying using HTTP reverse proxy.\n   28→type AIProxyService struct {\n   29→\tproxy               *httputil.ReverseProxy\n   30→\ttargetURL           *url.URL\n   31→\tapiKey              string\n   32→\tminBalanceThreshold float64\n   33→\ttokenUsageStore     *store.TokenUsageStore\n   34→\tbalanceStore \n[...truncated]\n\n[Assistant]\n基于代码分析，我来为您详细说明 Alterdo 后端系统的架构：\n\n## Alterdo 后端系统架构\n\n### 1. **整体架构模式**\n采用经典的 **分层架构 (Layered Architecture)**，基于 Go + Gin 框架构建：\n\n```\n┌─────────────────────────────────────────┐\n│          HTTP Request (Gin)             │\n├─────────────────────────────────────────┤\n│           Middleware Layer              │\n│  (Auth, CORS, Logger, Recovery, etc.)   │\n├─────────────────────────────────────────┤\n│            Handler Layer                │\n│    (HTTP Request Handlers)              │\n├─────────────────────────────────────────┤\n│            Service Layer                │\n│      (Business Logic)                   │\n├─────────────────────────────────────────┤\n│             Store Layer                 │\n│       (Data Access)                     │\n├─────────────────────────────────────────┤\n│    MySQL  │  Redis  │  Docker/gRPC      │\n└─────────────────────────────────────────┘\n```\n\n### 2. **核心组件**\n\n#### **中间件层 (Middleware)**\n- `auth.go` - JWT 认证\n- `apikey_auth.go` - API Key 认证\n- `cors.go` - 跨域处理\n- `logger.go` - 请求日志\n- `recovery.go` - 异常恢复\n- `request_id.go` - 请求追踪\n- `admin.go` - 管理员权限\n\n#### **处理器层 (Handler)**\n- `ai_proxy.go` - AI 代理请求处理\n- `auth.go` - 认证\n[...truncated]\n\n"

	t.Log("Starting L0 compression test with real LLM...")
	t.Logf("Raw content length: %d chars", len(rawContent))

	// 执行压缩
	compressed, err := compressor.Compress(ctx, rawContent, strategy)
	if err != nil {
		t.Fatalf("Compression failed: %v", err)
	}

	// 验证结果
	t.Logf("Compressed content length: %d chars", len(compressed))

	// 显示压缩后的内容
	t.Logf("\n=== Compressed Content ===\n%s", compressed)

	// 验证压缩后的内容非空且有意义
	if len(compressed) < 50 {
		t.Errorf("Compressed content too short: %d chars", len(compressed))
	}
}

// TestL0CompressWithSession 测试完整的Session L0压缩流程
func TestL0CompressWithSession(t *testing.T) {
	ctx := context.Background()

	// 创建 provider
	provider, err := openaicompat.New(
		openaicompat.WithBaseURL("https://dashscope.aliyuncs.com/compatible-mode/v1"),
		openaicompat.WithAPIKey("sk-8440a58764c846c88183bfec2d94d279"),
	)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// 加载提示词
	p := prompt.New("../../pkg/prompt/prompts")
	if err := p.Load(); err != nil {
		t.Fatalf("Failed to load prompts: %v", err)
	}

	// 创建配置
	cfg := &config.Config{
		Compressor: config.CompressorConfig{
			Model:       "qwen-plus",
			MaxTokens:   4096,
			Temperature: 0.3,
		},
		Context: config.ContextConfig{
			MaxPageTokens: 2000,
		},
	}

	// 创建L0压缩策略
	strategy := &CompressionStrategy{
		Skill:            p.L0Skill,
		CompressionModel: "qwen-plus",
		Timeout:          60 * time.Second,
	}

	// 创建压缩器
	compressor, err := NewLLMCompressor(ctx, provider, strategy, cfg, logger.NewNop(), p)
	if err != nil {
		t.Fatalf("Failed to create compressor: %v", err)
	}

	// 创建 Session
	session, err := NewSession("You are a helpful AI assistant.", cfg, compressor)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	defer session.Close()

	// 模拟多轮对话，添加足够多的消息触发L0压缩
	for i := range 5 {
		session.AddMessage(message.NewMessage(message.RoleUser,
			fmt.Sprintf("这是第 %d 条用户消息，用于测试L0压缩功能。我们需要添加足够多的内容来触发压缩。", i)))
		session.AddMessage(message.NewMessage(message.RoleAssistant,
			fmt.Sprintf("这是第 %d 条助手回复。我正在测试上下文管理系统的压缩功能。这个系统应该能够在消息过多时自动进行压缩。", i)))

		// 提交L2到L1
		if err := session.Commit(ctx); err != nil {
			t.Logf("Commit %d result: %v", i, err)
		}

		// 检查是否触发了L0压缩
		stats := session.GetStats()
		t.Logf("After commit %d: L1Pages=%d, L0Tokens=%d, L1Tokens=%d, L2Tokens=%d",
			i, stats.L1PageCount, stats.TotalL0Tokens, stats.TotalL1Tokens, stats.TotalL2Tokens)
	}

	// 验证最终状态
	stats := session.GetStats()
	t.Logf("\n=== Final Stats ===")
	t.Logf("L0 Tokens: %d", stats.TotalL0Tokens)
	t.Logf("L1 Tokens: %d", stats.TotalL1Tokens)
	t.Logf("L2 Tokens: %d", stats.TotalL2Tokens)
	t.Logf("L1 Pages: %d", stats.L1PageCount)

	// 获取上下文消息
	messages := session.GetContext()
	t.Logf("Total context messages: %d", len(messages))

	// 如果L0被创建，说明压缩成功
	if stats.TotalL0Tokens > 0 {
		t.Log("✓ L0 compression was triggered and completed successfully")
	} else {
		t.Log("✗ L0 compression was not triggered (not enough L1 pages)")
	}
}