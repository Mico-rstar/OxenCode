package context

import (
	"context"
	"time"
	"testing"

	openaicompat "charm.land/fantasy/providers/openaicompat"
	"github.com/yourname/oxencode/pkg/config"
	"github.com/yourname/oxencode/pkg/prompt"
)

func TestCompress(t *testing.T) {
	ctx := context.Background()
	provider, err := openaicompat.New(
		openaicompat.WithBaseURL("qwen3.5-plus"),
		openaicompat.WithAPIKey("sk-8440a58764c846c88183bfec2d94d279"),
	)
	if err != nil {
		t.Fatal(err)
	}
	cprs, err := NewLLMCompressor(ctx, provider, &config.Config{
		CompressorModel: "qwen3.5-plus",
		CompressorMaxTokens: 4096,
		CompressorTemperature: 0.7,
	}, nil)	
	if err != nil {
		t.Fatal(err)
	}
	p := prompt.New("../../pkg/prompt/prompts")
	if err := p.Load(); err != nil {
		t.Fatal(err)
	}
	strategy := &CompressionStrategy{
		Schema: p.L0Schema,
		MaxCompressionRate: 0.8,
		MinCompressionRate: 0.5,
		CompressionModel: "qwen3.5-plus",
		Timeout: 30 * time.Second,
	}
	res, err := cprs.Compress(ctx, `user: 
帮我在 prompts目录下写出这3个提示词模板，你应该首先通过 @docs/context-manage-idea.md 了解上下文系统设计思想
assistant:用户希望我根据 docs/context-manage-idea.md 中的设计思想，在 prompts 目录下创建三个提示词模板：

compressor_system.md - 压缩器的系统提示词
l0_schema.md - L0 级别的压缩 schema
l1_schema.md - L1 级别的压缩 schema
我已经读取了文档，现在我需要：

首先了解 prompts 目录的位置和结构
查看现有的提示词模板格式
根据文档中的设计创建这三个模板
让我先查看 prompts 目录的结构。

让我先了解 prompts 目录的位置和现有模板的格式。
Globpattern: "**/prompt*/**/*.md"
Found 3 files
Globpattern: "pkg/prompt/**/*.md"
Found 3 files
好的，prompts 目录在 pkg/prompt/prompts/。让我先查看现有模块的格式，然后创建压缩相关的提示词模板。

Read core.md
Read main_prompt.md
Read loader.go
Read core.md
Read main_prompt.md
Read loader.go
现在我了解了：

prompts 目录位于 pkg/prompt/prompts/
现有的提示词格式使用 XML 标签风格（如 <identity>, <core_principles> 等）
提示词支持 {{INCLUDE:modules/xxx.md}} 指令来包含其他模块
根据 docs/context-manage-idea.md 中的设计，我需要创建三个文件：

compressor_system.md - 压缩器的系统提示词，定义压缩模型的工作方式
l0_schema.md - L0 级别的压缩 schema，对多个 L1 Page 进行高层次压缩
l1_schema.md - L1 级别的压缩 schema，对一轮交互进行轻度压缩
让我根据文档中的设计创建这三个文件。

现在我了解了现有格式。让我根据设计文档创建这三个提示词模板。
`, strategy)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(res)
}