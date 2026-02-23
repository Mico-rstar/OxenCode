## Context

### 当前状态

OxenCode 的 ReAct loop 实现已支持工具调用和进度更新机制：

- **Agent 层** (`internal/agent/agent.go`): `ChatWithToolsWithProgress` 方法通过 `ProgressUpdate` channel 发送进度
- **TUI 层** (`internal/ui/handlers.go`): `waitForAgentProgress` 处理进度更新，`handleReActStep` 处理 ReAct 步骤
- **底层库** (fantasy v0.8.1): 已提供 `OnReasoningStart`, `OnReasoningDelta`, `OnReasoningEnd` 回调

### 问题

虽然 fantasy 库支持 reasoning 回调，但 `ChatWithToolsWithProgress` 只注册了工具相关回调：
```go
// 当前只注册了这些
OnToolCall:   func(toolCall fantasy.ToolCallContent) error { ... }
OnToolResult: func(result fantasy.ToolResultContent) error { ... }
OnFinish:     func(result *fantasy.AgentResult) { ... }
```

TUI 已支持显示 `thought` 类型的 ReAct 步骤（见 `handlers.go:346-351`），但来源是假的 "echo 用户输入"（`agent.go:585`）。

---

## Goals / Non-Goals

**Goals:**

1. 捕获真实的 LLM 推理内容（Chain of Thought）
2. 实时流式传输推理内容到 TUI
3. 在 TUI 中以视觉区分的方式展示推理过程
4. 保持向后兼容，不破坏现有功能

**Non-Goals:**

1. 修改 `ProgressUpdate` 类型结构（复用现有的 `thought` 类型）
2. 修改 `ReActStepMsg` 消息结构（TUI 已支持）
3. 实现推理内容的持久化存储（仅实时展示）
4. 修改用户提示词来强制生成推理内容（由 provider/model 决定）

---

## Decisions

### 1. 注册 Reasoning 回调

**决策**: 在 `ChatWithToolsWithProgress` 中添加 `OnReasoningDelta` 回调

```go
result, err := a.agent.Stream(ctx, fantasy.AgentStreamCall{
    Prompt:       userMessage,
    Messages:     messages,
    ActiveTools:  toolNames,
    // 新增 reasoning 回调
    OnReasoningDelta: func(id, text string) error {
        sendProgress("thought", text, "")
        return nil
    },
    // 保留现有回调
    OnToolCall:   func(toolCall fantasy.ToolCallContent) error { ... },
    OnToolResult: func(result fantasy.ToolResultContent) error { ... },
    OnFinish:     func(result *fantasy.AgentResult) { ... },
})
```

**理由**:
- `OnReasoningDelta` 在每次推理增量时触发，支持流式传输
- 不需要 `OnReasoningStart/End`，因为 TUI 只关心内容本身
- `text` 参数直接包含推理文本，无需额外处理

### 2. 移除假的 Thought

**决策**: 删除 `agent.go:584-585` 的假 thought 发送

```go
// 删除这两行
// sendProgress("thought", "User is asking: "+userMessage, "")
```

**理由**:
- 这个 "thought" 只是 echo 用户输入，不是真实的推理
- 会被真实的 LLM reasoning 替代
- 减少噪音

### 3. TUI 无需修改

**决策**: TUI 层代码无需修改

**理由**:
- `handleReActStep` 已经处理 `thought` 类型（`handlers.go:430-431`）
- `renderMessage` 已经支持渲染 ReAct 步骤
- `StateThinking` 状态已存在，只需确保正确触发

**验证需求**:
- 确认推理内容的样式足够区分（可能需要调整 `styles.go`）
- 确认推理内容在工具调用之前显示（时间顺序由 Agent 保证）

### 4. Provider 兼容性处理

**决策**: 不检查 provider 是否支持 reasoning，让回调自然失效

**理由**:
- 如果 provider 不支持 reasoning，回调永远不会被触发
- 不需要额外的检测逻辑
- 保持代码简洁

**风险**: 某些 provider 可能触发空回调 → 需要测试验证

---

## Implementation Strategy

### Phase 1: Agent 层修改

**文件**: `internal/agent/agent.go`

**修改点**:
1. 添加 `buildProviderOptions` 辅助函数来构建 ProviderOptions
2. 在 `NewAgent` 中使用 `fantasy.WithProviderOptions` 设置配置
3. 在 `ChatWithToolsWithProgress` 的 `fantasy.AgentStreamCall` 中添加 `OnReasoningDelta`
4. 删除假的 `sendProgress("thought", ...)`

**代码变更**:

```go
// 1. 新增辅助函数
func buildProviderOptions(cfg *config.Config, log logger.Logger) fantasy.ProviderOptions {
    if cfg.Provider == config.ProviderAnthropic && cfg.ThinkingBudget > 0 {
        trueVal := true
        budgetTokens := int64(cfg.ThinkingBudget)
        anthropicOpts := &anthropic.ProviderOptions{
            SendReasoning: &trueVal,
            Thinking: &anthropic.ThinkingProviderOption{
                BudgetTokens: budgetTokens,
            },
        }
        log.Info("Extended thinking enabled", "budget_tokens", cfg.ThinkingBudget)
        return anthropic.NewProviderOptions(anthropicOpts)
    }
    return nil
}

// 2. 在 NewAgent 中应用
providerOpts := buildProviderOptions(cfg, log)
agentOpts := []fantasy.AgentOption{
    fantasy.WithMaxOutputTokens(int64(cfg.MaxTokens)),
    fantasy.WithTemperature(cfg.Temperature),
    fantasy.WithTools(fantasyTools...),
}
if providerOpts != nil {
    agentOpts = append(agentOpts, fantasy.WithProviderOptions(providerOpts))
}
agent := fantasy.NewAgent(model, agentOpts...)

// 3. 在 ChatWithToolsWithProgress 中添加回调
result, err := a.agent.Stream(ctx, fantasy.AgentStreamCall{
    Prompt:       userMessage,
    Messages:     messages,
    ActiveTools:  toolNames,
    OnReasoningDelta: func(id, text string) error {
        a.logger.Debug("Reasoning delta", "length", len(text))
        sendProgress("thought", text, "")
        return nil
    },
    // ... 现有回调保持不变
})
```

### Phase 2: Config 层修改

**文件**: `pkg/config/config.go`, `config.example.toml`

**修改点**:
1. 添加 `ThinkingBudget int` 配置字段
2. 设置默认值为 0（不启用）
3. 在配置示例中添加说明

### Phase 3: TUI 样式验证（可选）

**文件**: `internal/ui/styles.go`

**可能的调整**:
- 确保 `thought` 类型的 ReAct 步骤有明显的视觉样式
- 考虑使用灰色、斜体或特殊前缀（如 "🤔 Thinking:"）

### Phase 4: 测试

**手动测试场景**:
1. 使用支持 reasoning 的模型（如 Claude 3.7 Sonnet with extended thinking）
2. 发送需要工具调用的请求
3. 观察 TUI 中是否显示推理内容
4. 验证时间顺序：reasoning → action → observation

---

## Data Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│  完整的 ReAct 循环数据流（带推理内容）                              │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  User Input                                                         │
│      │                                                              │
│      ▼                                                              │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  fantasy.Stream()                                             │  │
│  │    ├─ OnReasoningDelta("I need to search for...")            │  │
│  │    ├─ OnReasoningDelta("Let me use grep tool...")            │  │
│  │    ├─ OnToolCall(grep("pattern", "*.go"))                    │  │
│  │    └─ OnToolResult("Found 3 matches...")                     │  │
│  └──────────────────────────────────────────────────────────────┘  │
│      │                                                              │
│      ▼                                                              │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  ProgressUpdate Channel                                       │  │
│  │    ├─ Type:"thought", Content:"I need to..."                 │  │
│  │    ├─ Type:"thought", Content:"Let me use..."                │  │
│  │    ├─ Type:"action", Content:"grep(...)", ToolName:"grep"    │  │
│  │    └─ Type:"observation", Content:"Found 3...", ToolName:"grep" │  │
│  └──────────────────────────────────────────────────────────────┘  │
│      │                                                              │
│      ▼                                                              │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  TUI: waitForAgentProgress()                                  │  │
│  │    └─ handleReActStep() → AddReActStep("thought", ...)       │  │
│  └──────────────────────────────────────────────────────────────┘  │
│      │                                                              │
│      ▼                                                              │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  TUI Display                                                  │  │
│  │    🤔 I need to search for...                                │  │
│  │    🤔 Let me use grep tool...                                │  │
│  │    🔧 grep("pattern", "*.go")                                │  │
│  │    ✓ Found 3 matches...                                      │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Risks / Trade-offs

### Risk 1: Provider 不支持 Reasoning

**风险**: 某些 provider（如 OpenAI）可能不输出 reasoning 内容

**缓解**:
- 回调自然失效，不影响功能
- 用户看不到推理过程，但工具调用仍正常工作

### Risk 2: Reasoning 内容过长

**风险**: 某些模型的推理内容可能非常长（数万 tokens）

**缓解**:
- TUI 已有滚动支持
- 未来可考虑截断或折叠（非本次 scope）

### Risk 3: 性能影响

**风险**: 频繁发送 `ProgressUpdate` 可能影响性能

**评估**: 低风险
- 推理内容通常是短文本片段
- Channel 已缓冲（size=100）
- TUI 已有高效的更新机制

### Trade-off: 简洁 vs 信息量

**选择**: 展示完整推理内容，即使可能很长

**理由**:
- 推理过程是核心价值
- 用户可以滚动查看
- 未来可添加折叠功能

---

## Alternatives Considered

### Alternative 1: 新增 `reasoning` 类型到 `ProgressUpdate`

**拒绝理由**:
- `thought` 类型已存在且语义匹配
- TUI 已支持，无需修改
- 遵循"最小变更"原则

### Alternative 2: 累积推理内容后一次性发送

**拒绝理由**:
- 失去流式展示的实时性
- 用户需要等待较长时间才能看到内容
- 不符合"think → action"的即时反馈体验

### Alternative 3: 将推理内容存储到消息对象中

**拒绝理由**:
- 推理内容是临时的过程展示，不需要持久化
- 增加消息对象复杂度
- 当前的 `AddReActStep` 足够
