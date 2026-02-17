# ReAct 循环实现文档

本文档描述了 OxenCode Agent 中 ReAct (Reasoning + Acting) 循环的实现。

---

## 目录

1. [概述](#概述)
2. [ReAct 循环原理](#react-循环原理)
3. [实现架构](#实现架构)
4. [API 参考](#api-参考)
5. [执行流程](#执行流程)
6. [使用示例](#使用示例)
7. [限制与注意事项](#限制与注意事项)

---

## 概述

ReAct 循环是一种让 AI Agent 通过"思考-行动-观察"的迭代过程来解决复杂任务的范式。在 OxenCode 中，ReAct 循环使 Agent 能够：

- 理解用户的请求
- 决定需要使用哪些工具
- 执行工具调用
- 观察工具结果
- 基于结果继续下一步行动
- 最终给出答案

### 核心特性

- **自动化决策**: Agent 自动决定何时以及如何使用工具
- **迭代推理**: 支持多轮工具调用，逐步解决复杂任务
- **错误恢复**: 工具调用失败时可以重试或调整策略
- **安全限制**: 最大迭代次数限制，防止无限循环

---

## ReAct 循环原理

### 三步循环

ReAct 循环由三个步骤组成：

```
┌─────────────────────────────────────────────────┐
│  1. Thought (思考)                               │
│     - 分析当前状态和用户请求                      │
│     - 决定下一步行动                              │
│     - 选择合适的工具                              │
└──────────────────┬──────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────┐
│  2. Action (行动)                                │
│     - 执行工具调用                                │
│     - 传递正确的参数                              │
│     - 记录调用信息                                │
└──────────────────┬──────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────┐
│  3. Observation (观察)                           │
│     - 接收工具执行结果                            │
│     - 分析结果是否满足需求                        │
│     - 决定是继续还是结束                          │
└──────────────────┬──────────────────────────────┘
                   │
                   ▼
            ┌──────┴───────┐
            │                 │
      需要继续?         任务完成
            │                 │
            ▼                 ▼
      回到 Thought      返回最终答案
```

### 实际示例

**用户请求**: "找出所有包含 'error' 的 Go 文件"

```
Iteration 1:
  Thought: "我需要先找出所有 Go 文件"
  Action: Glob(pattern="*.go")
  Observation: "找到 15 个 .go 文件"

Iteration 2:
  Thought: "现在在这些文件中搜索 'error'"
  Action: Grep(pattern="error", file_pattern="*.go")
  Observation: "在 8 个文件中找到 23 处匹配"

Iteration 3:
  Thought: "搜索完成，现在总结结果"
  Action: (无工具调用，直接生成回答)
  Final Answer: "找到以下文件包含 error..."
```

---

## 实现架构

### ChatWithTools 方法

核心实现位于 [internal/agent/agent.go](../internal/agent/agent.go)：

```go
func (a *Agent) ChatWithTools(ctx context.Context, userMessage string) (string, error)
```

### 流程控制

```
开始
  │
  ├─ 创建新消息
  ├─ 添加用户消息到历史
  │
  └─ 进入循环 (最多 10 次)
       │
       ├─ 构建消息列表（包含工具结果）
       ├─ 调用 LLM 获取响应
       ├─ 解析响应内容
       │  │
       │  ├─ 有文本内容？
       │  │  └─ 累积到响应中
       │  │
       │  └─ 有工具调用？
       │     ├─ 执行工具
       │     ├─ 添加工具结果到历史
       │     └─ 标记 hasToolCalls = true
       │
       └─ 检查是否需要继续
          │
          ├─ hasToolCalls = true
          │  └─ 继续下一轮
          │
          └─ hasToolCalls = false
             └─ 返回最终响应
```

### 关键组件

#### 1. 消息构建

`buildMessagesWithTools()` - 构建包含工具调用历史的消息列表：

```go
func (a *Agent) buildMessagesWithTools() []fantasy.Message
```

**转换规则**:
- `RoleUser` → `fantasy.NewUserMessage(content)`
- `RoleAssistant` → `fantasy.Message{Role: Assistant, Content: ...}`
- `RoleSystem` → `fantasy.NewSystemMessage(content)`
- `RoleTool` → `fantasy.Message{Role: User, Content: ...}` (工具结果作为用户消息)

#### 2. 工具执行

`executeToolCall()` - 执行单个工具调用：

```go
func (a *Agent) executeToolCall(ctx context.Context, toolCall fantasy.ToolCallContent) (string, error)
```

**流程**:
1. 解析工具输入参数 (JSON)
2. 调用 `ExecuteTool()`
3. 返回结果或错误

#### 3. 循环继续

`ContinueReAct()` - 继续执行 ReAct 循环（用于异步场景）：

```go
func (a *Agent) ContinueReAct(ctx context.Context, currentMsg message.Message) (string, error)
```

---

## API 参考

### ChatWithTools

执行完整的 ReAct 循环对话。

```go
func (a *Agent) ChatWithTools(ctx context.Context, userMessage string) (string, error)
```

**参数**:
- `ctx`: 上下文，用于取消和超时控制
- `userMessage`: 用户消息

**返回**:
- `string`: AI 的最终响应
- `error`: 错误信息（如果有）

**特性**:
- 自动进行多轮工具调用
- 最多 10 次迭代（防止无限循环）
- 每次工具调用都会记录到消息历史

### ContinueReAct

继续 ReAct 循环（用于异步/流式场景）。

```go
func (a *Agent) ContinueReAct(ctx context.Context, currentMsg message.Message) (string, error)
```

**参数**:
- `ctx`: 上下文
- `currentMsg`: 当前消息对象

**返回**:
- `string`: 如果有内容表示完成，空字符串表示需要继续
- `error`: 错误信息

### ExecuteTool

执行单个工具（已在工具集成文档中描述）。

```go
func (a *Agent) ExecuteTool(ctx context.Context, toolName string, input map[string]any) (string, error)
```

---

## 执行流程

### 详细步骤

```go
// 1. 初始化
currentMsg := NewStreamingMessage(RoleAssistant)
currentMsg.AddReActStep("thought", "User is asking: " + userMessage)

// 2. 添加用户消息到历史
userMsg := NewMessage(RoleUser, userMessage)
history = append(history, userMsg)

// 3. 开始循环
for iteration := 0; iteration < maxIterations; iteration++ {
    // 3.1 构建消息（包含之前的工具结果）
    messages := buildMessagesWithTools()

    // 3.2 获取可用工具
    toolNames := toolRegistry.Names()

    // 3.3 调用 LLM
    result := agent.Generate(ctx, AgentCall{
        Prompt: userMessage,
        Messages: messages,
        ActiveTools: toolNames,  // 告诉 LLM 有哪些工具可用
    })

    // 3.4 解析响应
    content := result.Response.Content
    hasToolCalls := false
    responseText := ""

    for _, c := range content {
        switch v := c.(type) {
        case TextContent:
            // 文本响应
            responseText += v.Text
            currentMsg.AppendContent(v.Text)

        case ToolCallContent:
            // 工具调用
            hasToolCalls = true
            currentMsg.AddToolCall(v.ToolName, {...})

            // 执行工具
            output, err := executeToolCall(ctx, v)
            if err != nil {
                currentMsg.UpdateToolCall(v.ToolName, err.Error(), StatusError, err.Error())
            } else {
                currentMsg.UpdateToolCall(v.ToolName, output, StatusCompleted, "")
                currentMsg.AddReActStep("observation", output)
            }

            // 添加工具结果到历史（用于下一轮）
            toolMsg := NewMessage(RoleTool, output)
            history = append(history, toolMsg)
        }
    }

    // 3.5 检查是否继续
    if !hasToolCalls {
        // 没有 tool calls，任务完成
        currentMsg.Complete()
        assistantMsg := NewMessage(RoleAssistant, responseText)
        history = append(history, assistantMsg)
        return responseText, nil
    }

    // 有 tool calls，继续下一轮
}

// 4. 达到最大迭代次数
return "", fmt.Errorf("ReAct loop reached maximum iterations")
```

### LLM 工具调用格式

LLM 通过返回特殊的 `ToolCallContent` 来请求工具调用：

```json
{
  "tool_call_id": "call_123",
  "tool_name": "grep",
  "input": "{\"pattern\": \"error\", \"file_pattern\": \"*.go\"}"
}
```

工具结果作为 `RoleTool` 消息返回给 LLM：

```json
{
  "role": "user",
  "content": "main.go:42: return err\nagent.go:15: err != nil"
}
```

---

## 使用示例

### 基本使用

```go
// 创建 Agent
agent, err := NewAgent(cfg)
if err != nil {
    log.Fatal(err)
}

ctx := context.Background()

// 简单对话（不需要工具）
response, err := agent.ChatWithTools(ctx, "1+1等于几？")
fmt.Println(response)

// 需要工具的对话
response, err = agent.ChatWithTools(ctx, "列出所有 Go 文件")
fmt.Println(response)
```

### 复杂任务

```go
// 这个任务需要多个工具
response, err := agent.ChatWithTools(ctx, `
    请分析以下代码：
    1. 找出所有 Go 文件
    2. 搜索包含 "TODO" 的文件
    3. 读取第一个包含 TODO 的文件内容
`)

if err != nil {
    log.Printf("Error: %v", err)
} else {
    fmt.Println("分析结果:", response)
}
```

### 异步使用（流式）

```go
// 创建消息对象
currentMsg := message.NewStreamingMessage(message.RoleAssistant)

// 第一轮
result, err := agent.ContinueReAct(ctx, currentMsg)
if result == "" {
    // 还有工具调用，继续
    result, err = agent.ContinueReAct(ctx, currentMsg)
}
```

---

## 限制与注意事项

### 1. 最大迭代次数

默认最大迭代次数为 **10**，超过后返回错误：

```go
return "", fmt.Errorf("ReAct loop reached maximum iterations (10)")
```

这防止了无限循环，但可能限制复杂任务的完成。

### 2. 上下文窗口

每次工具调用的结果都会添加到消息历史中，可能消耗大量 token：

```go
// 每次工具调用后
toolResultMsg := NewMessage(RoleTool, output)  // 可能有几千字符
history = append(history, toolResultMsg)
```

**建议**:
- 工具返回简洁的结果
- 使用 limit/offset 分页读取大文件
- 避免在循环中读取大量文件

### 3. 工具错误处理

工具调用失败时，错误信息会作为工具结果返回：

```go
if err != nil {
    toolResultMsg := NewMessage(RoleTool, fmt.Sprintf("Error: %v", err))
    history = append(history, toolResultMsg)
}
```

LLM 需要根据错误信息决定是重试、尝试其他方法还是放弃。

### 4. 并发限制

当前实现是串行的：
- 一次只执行一个工具
- 等待工具完成后才继续

**未来改进**: 支持并行执行独立工具。

### 5. 工具参数验证

工具参数在执行前验证：

```go
if err := tool.Validate(input); err != nil {
    return "", fmt.Errorf("parameter validation failed: %w", err)
}
```

LLM 可能需要多次尝试才能提供正确的参数。

---

## 性能考虑

### 每次迭代的成本

1. **LLM 调用**: 每次迭代都调用 LLM API
2. **工具执行**: 取决于具体工具
   - Glob: 快速 (~10ms)
   - Grep: 中等 (~50-500ms)
   - Read: 快速 (~1-10ms)

### 优化建议

1. **精确的提示**: 明确告诉 Agent 需要什么，减少不必要的工具调用
2. **合理的工具使用**: 避免在循环中重复执行相同工具
3. **结果限制**: 使用 limit/offset 避免读取过多数据
4. **缓存考虑**: 未来可以添加工具结果缓存

---

## 测试

### 测试覆盖

位于 [internal/agent/agent_test.go](../internal/agent/agent_test.go):

- `TestChatWithTools` - 测试 ReAct 循环对话

### 运行测试

```bash
# 运行 ReAct 测试
go test -v ./internal/agent/... -run TestChatWithTools

# 运行所有 Agent 测试
go test ./internal/agent/...
```

---

## 未来改进

### Phase 3.4: 增强

1. **并行工具执行**: 同时执行多个独立工具
2. **工具链**: 将一个工具的输出作为另一个的输入
3. **智能终止**: 更早判断任务完成
4. **错误恢复**: 自动重试失败的工具调用

### Phase 4: TUI 集成

1. **实时显示**: 在 TUI 中显示工具调用过程
2. **进度指示**: 显示当前迭代进度
3. **交互式控制**: 允许用户中断或继续循环
4. **可视化 ReAct**: 图形化显示 Thought-Action-Observation 流程

---

## 相关文档

- [工具集成文档](tool-integration.md) - 工具注册和执行
- [工具行为文档](tools-behavior.md) - 工具使用参考
- [工具系统设计](wip/tool.md) - 架构设计

---

**最后更新**: 2026-02-17
**版本**: v0.1.0
