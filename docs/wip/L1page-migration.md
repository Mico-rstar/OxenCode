# L1 上下文预处理重构

## Context

### 问题背景
当前L1压缩策略调用LLM对消息进行压缩，这增加了不必要的开销。用户分析认为：

1. OxenCode一轮任务中，低密度信息集中在**工具调用结果**和**Assistant消息**
2. L1不需要真正执行压缩，只需对这两类内容进行截断处理
3. 保留原始消息序列顺序

### 设计原则
1. **将差异延迟处理**: L0/L1/L2使用统一的处理流程，差异通过配置控制
2. **配置集中管理**: 所有参数通过 `Config` 配置，避免分散在代码中
3. **兜底策略**: Page有最大token限制，防止单任务上下文过长

### 需求确认
- ✅ L1 **不调用LLM压缩**
- ✅ 保留原始消息序列顺序
- ✅ 工具调用结果/Assistant消息：超过阈值截断 + `[...truncated]` 标记
- ✅ 所有参数通过 `Config` 配置
- ✅ Page最大token限制（兜底策略）

---

## Design

### 核心架构

```
┌─────────────────────────────────────────────────────────────────┐
│                         Config                                   │
│  - L1MaxToolOutputLength: 1000                                  │
│  - L1MaxAssistantLength: 2000                                   │
│  - MaxPageTokens: 50000 (兜底)                                  │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Session.AddMessage()                          │
│  检查Page token数，超过限制时触发分页                             │
└─────────────────────────────────────────────────────────────────┘
                                │
              ┌─────────────────┴─────────────────┐
              ▼                                   ▼
    ┌─────────────────┐                 ┌─────────────────┐
    │  旧消息 (1/2)   │                 │  新消息 (1/2)   │
    │  → L1处理       │                 │  → 保留L2级     │
    └─────────────────┘                 └─────────────────┘
```

### 核心变更

#### 1. 扩展 `Config` (`pkg/config/config.go`)

```go
// Config 应用配置
type Config struct {
    // ... 现有配置 ...

    // L1 预处理配置
    L1MaxToolOutputLength int `mapstructure:"l1_max_tool_output_length"` // 工具输出最大长度
    L1MaxAssistantLength  int `mapstructure:"l1_max_assistant_length"`   // Assistant消息最大长度

    // Page 兜底配置
    MaxPageTokens int `mapstructure:"max_page_tokens"` // 单Page最大token数
}
```

默认值：
```go
v.SetDefault("l1_max_tool_output_length", 1000)
v.SetDefault("l1_max_assistant_length", 2000)
v.SetDefault("max_page_tokens", 50000)  // 约200KB文本
```

#### 2. 简化 `CompressionStrategy` (`internal/context/types.go`)

移除硬编码的默认值，改为从Config读取：

```go
// CompressionStrategy 压缩策略配置
type CompressionStrategy struct {
    MaxCompressionRate float64       `json:"max_compression_rate"`
    MinCompressionRate float64       `json:"min_compression_rate"`
    Schema             string        `json:"schema"`
    Skill              string        `json:"skill"`
    CompressionModel   string        `json:"compression_model"`
    Timeout            time.Duration `json:"timeout"`

    // 截断配置（从Config读取）
    MaxToolOutputLength int `json:"max_tool_output_length"`
    MaxAssistantLength  int `json:"max_assistant_length"`
}

// NewCompressionStrategy 从Config创建策略
func NewCompressionStrategy(pageType PageType, cfg *config.Config) *CompressionStrategy {
    switch pageType {
    case PageTypeL1:
        return &CompressionStrategy{
            MaxToolOutputLength: cfg.L1MaxToolOutputLength,
            MaxAssistantLength:  cfg.L1MaxAssistantLength,
            // ... 其他配置
        }
    case PageTypeL0, PageTypeL2:
        return &CompressionStrategy{
            MaxToolOutputLength: 0, // 不截断
            MaxAssistantLength:  0,
            // ... 其他配置
        }
    }
}
```

#### 3. Page分页逻辑 (`internal/context/session.go`)

```go
// AddMessage 添加消息到Session
func (s *Session) AddMessage(msg message.Message) {
    s.mu.Lock()
    defer s.mu.Unlock()

    // 确保有当前的L2 page
    if len(s.L2Pages) == 0 {
        s.L2Pages = append(s.L2Pages, NewL2Page())
    }

    currentL2 := s.L2Pages[0]
    currentL2.AddMessage(msg)

    // 检查是否超过token限制
    if currentL2.GetTokenCount() > s.cfg.MaxPageTokens {
        s.splitL2Page()
    }
}

// splitL2Page 分页：旧消息→L1，新消息→L2
func (s *Session) splitL2Page() {
    currentL2 := s.L2Pages[0]
    messages := currentL2.Messages

    // 按1/2分割
    mid := len(messages) / 2
    oldMessages := messages[:mid]
    newMessages := messages[mid:]

    // 旧消息创建L1 Page
    l1Page := NewPage(PageTypeL1, NewCompressionStrategy(PageTypeL1, s.cfg))
    l1Page.Messages = oldMessages
    l1Page.Preprocess()
    s.L1Pages = append([]*Page{l1Page}, s.L1Pages...)

    // 新消息保留为L2
    newL2 := NewL2Page()
    newL2.Messages = newMessages
    s.L2Pages[0] = newL2

    s.logger.Info("L2 page split due to token limit",
        "old_messages", len(oldMessages),
        "new_messages", len(newMessages))
}
```

#### 4. `Page.Preprocess()` (`internal/context/page.go`)

```go
const TruncatedMarker = "\n[...truncated]"

// Preprocess 预处理消息，根据Strategy配置截断
func (p *Page) Preprocess() {
    if p.Strategy == nil {
        return
    }

    processed := make([]message.Message, len(p.Messages))
    for i, msg := range p.Messages {
        processed[i] = p.truncateMessage(msg)
    }
    p.ProcessedMessages = processed
}

// truncateMessage 根据策略截断消息
func (p *Page) truncateMessage(msg message.Message) message.Message {
    result := msg

    if msg.Role == message.RoleAssistant && p.Strategy.MaxAssistantLength > 0 {
        if len(msg.Content) > p.Strategy.MaxAssistantLength {
            result.Content = msg.Content[:p.Strategy.MaxAssistantLength] + TruncatedMarker
        }
        for j, step := range result.ReActLoop {
            if step.ToolCall != nil && p.Strategy.MaxToolOutputLength > 0 {
                if len(step.ToolCall.Output) > p.Strategy.MaxToolOutputLength {
                    result.ReActLoop[j].ToolCall.Output =
                        step.ToolCall.Output[:p.Strategy.MaxToolOutputLength] + TruncatedMarker
                }
            }
        }
    }

    if msg.Role == message.RoleTool && p.Strategy.MaxToolOutputLength > 0 {
        if len(msg.Content) > p.Strategy.MaxToolOutputLength {
            result.Content = msg.Content[:p.Strategy.MaxToolOutputLength] + TruncatedMarker
        }
    }

    return result
}
```

#### 5. `Page.Render()` (`internal/context/page.go`)

```go
// Render 渲染页面内容为可读文本
func (p *Page) Render() string {
    if p.Type == PageTypeL0 && p.Content != "" {
        return p.Content
    }

    messages := p.Messages
    if p.ProcessedMessages != nil {
        messages = p.ProcessedMessages
    }

    var sb strings.Builder
    for _, msg := range messages {
        switch msg.Role {
        case message.RoleUser:
            sb.WriteString("[User]\n")
            sb.WriteString(msg.Content)
            sb.WriteString("\n\n")
        case message.RoleAssistant:
            sb.WriteString("[Assistant]\n")
            sb.WriteString(msg.Content)
            sb.WriteString("\n\n")
            for _, step := range msg.ReActLoop {
                if step.ToolCall != nil {
                    sb.WriteString(fmt.Sprintf("[Tool: %s]\n", step.ToolCall.Name))
                    sb.WriteString(step.ToolCall.Output)
                    sb.WriteString("\n\n")
                }
            }
        case message.RoleTool:
            sb.WriteString("[Tool Result]\n")
            sb.WriteString(msg.Content)
            sb.WriteString("\n\n")
        }
    }
    return sb.String()
}
```

---

## Implementation Tasks

### Task 1: 配置扩展
- [ ] 在 `Config` 添加 `L1MaxToolOutputLength`, `L1MaxAssistantLength`, `MaxPageTokens`
- [ ] 设置默认值

### Task 2: 策略重构
- [ ] 修改 `CompressionStrategy` 从Config读取截断配置
- [ ] 添加 `NewCompressionStrategy()` 工厂函数

### Task 3: 分页逻辑
- [ ] 在 `Session.AddMessage()` 添加token检查
- [ ] 实现 `splitL2Page()` 分页方法

### Task 4: 预处理和渲染
- [ ] 实现 `Page.Preprocess()`
- [ ] 重构 `Page.Render()` 输出可读文本

### Task 5: 测试
- [ ] 添加 `TestSplitL2Page` 单元测试
- [ ] 添加 `TestPreprocess` 单元测试
- [ ] 添加 `TestRender` 单元测试

---

## Key Files

| 文件 | 变更内容 |
|------|----------|
| `pkg/config/config.go` | 添加 L1截断配置和MaxPageTokens |
| `internal/context/types.go` | 重构 CompressionStrategy |
| `internal/context/page.go` | 添加 Preprocess(), 重构 Render() |
| `internal/context/session.go` | 添加分页逻辑 |

---

## Verification

```bash
# 运行测试
go test ./internal/context/... -v

# 验证构建
go build ./...
```

### 预期效果
1. **配置集中**: 所有参数通过Config配置
2. **兜底保护**: Page超过token限制自动分页
3. **统一流程**: L0/L1/L2使用相同处理逻辑
4. **可读输出**: Render输出带角色标记的文本