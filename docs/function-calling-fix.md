# Function Calling 修复

## 问题描述

在使用 ReAct 循环时，agent 没有触发工具调用，直接返回文本响应。

日志显示：
```
{"L":"INFO","N":"agent","M":"Starting ChatWithTools","messageLength":36}
{"L":"INFO","N":"agent","M":"No tool calls, task completed"}
```

## 根本原因

工具没有注册到 fantasy agent。虽然我们在 `ChatWithTools` 中传递了 `ActiveTools`，但这只是告诉 agent 哪些工具是激活的，并没有将工具的 schema 注册到 agent 中。

Fantasy agent 需要通过 `WithTools` 选项注册工具的完整定义（名称、描述、参数 schema）。

## 解决方案

### 1. 创建工具适配器

创建了 `internal/tools/adapter.go`，将我们的 `Tool` 接口适配为 `fantasy.AgentTool` 接口：

```go
// AgentToolAdapter 将我们的 Tool 适配为 fantasy.AgentTool
type AgentToolAdapter struct {
    tool  Tool
    info  fantasy.ToolInfo
    opts  fantasy.ProviderOptions
}

// NewAgentToolAdapter 创建适配器
func NewAgentToolAdapter(tool Tool) fantasy.AgentTool

// ToolsToAgentTools 批量转换
func ToolsToAgentTools(tools []Tool) []fantasy.AgentTool
```

### 2. 修改 Agent 创建流程

在 `internal/agent/agent.go` 的 `NewAgent` 函数中：

1. 先创建工具实例
2. 注册到工具注册表
3. 转换为 fantasy.AgentTool
4. 在创建 agent 时使用 `fantasy.WithTools` 注册

```go
// 注册 P0 工具
globTool := tools.NewGlobTool(env, log)
grepTool := tools.NewGrepTool(env, log)
readTool := tools.NewReadTool(env, log)

registry.Register(globTool)
registry.Register(grepTool)
registry.Register(readTool)

// 将工具转换为 fantasy.AgentTool
fantasyTools := tools.ToolsToAgentTools(registry.List())

// 创建 Agent 并注册工具
agent := fantasy.NewAgent(
    model,
    fantasy.WithMaxOutputTokens(int64(cfg.MaxTokens)),
    fantasy.WithTemperature(cfg.Temperature),
    fantasy.WithTools(fantasyTools...),  // 关键：注册工具
)
```

## 测试

添加了适配器测试 (`TestAgentToolAdapter`)，覆盖：
- Glob 工具适配
- Grep 工具适配
- 无效 JSON 输入
- 缺少必填参数

## 验证

现在运行 TUI 并测试：
```
user: 当前文件夹有几个文件
```

应该能看到：
1. Agent 生成工具调用 (glob 工具)
2. TUI 显示 ReAct 步骤（Thought → Action → Observation）
3. 返回文件计数结果

## 相关文件

- [internal/tools/adapter.go](../internal/tools/adapter.go) - 工具适配器
- [internal/agent/agent.go](../internal/agent/agent.go) - Agent 创建（第 56-91 行）
- [internal/tools/p0tools_test.go](../internal/tools/p0tools_test.go) - 适配器测试

---

# UI 渲染工具调用修复

## 问题描述

工具调用成功执行（从日志可见），但 TUI 界面没有显示工具调用的 ReAct 步骤。

日志显示：
```
{"L":"INFO","N":"agent","M":"Starting ChatWithTools","messageLength":36}
{"L":"INFO","N":"agent.tool.glob","M":"Glob completed","matchCount":1}
{"L":"INFO","N":"agent","M":"No tool calls, task completed"}
```

但 UI 上看不到 Thought/Action/Observation 的显示。

## 根本原因

`ChatWithTools` 在 goroutine 中同步执行，只在完成后返回 `ChatWithToolsCompleteMsg`。虽然在内部消息对象上添加了 ReAct 步骤，但没有发送进度更新到 TUI，导致 UI 无法实时渲染。

## 解决方案

### 1. 添加进度更新类型

在 `internal/agent/agent.go` 中添加：

```go
// ProgressUpdate 进度更新类型
type ProgressUpdate struct {
    Type     string // "thought", "action", "observation", "content", "error"
    Content  string
    ToolName string // 仅用于 action 类型
}
```

### 2. 创建带进度更新的方法

新增 `ChatWithToolsWithProgress` 方法，接受一个 progress channel：

```go
func (a *Agent) ChatWithToolsWithProgress(
    ctx context.Context,
    userMessage string,
    progressCh chan<- ProgressUpdate,
) (string, error)
```

在执行过程中发送进度更新：
- **thought**: 发送思考步骤
- **action**: 发送工具调用动作
- **observation**: 发送工具执行结果
- **content**: 发送文本内容
- **error**: 发送错误信息

### 3. 更新 TUI 处理

修改 `internal/ui/handlers.go` 的 `handleChatWithToolsStart`：

1. 创建带缓冲的 progress channel
2. 启动两个 goroutine：
   - Goroutine 1: 执行 `ChatWithToolsWithProgress`
   - Goroutine 2: 监听 progress channel 并转发为 `ReActStepMsg`

```go
progressCh := make(chan agent.ProgressUpdate, 100)

return m, tea.Batch(
    // 执行 ChatWithTools
    func() tea.Msg {
        response, err := m.agent.ChatWithToolsWithProgress(m.ctx, msg.UserContent, progressCh)
        close(progressCh)
        // 返回完成消息
    },
    // 监听进度更新
    func() tea.Msg {
        for update := range progressCh {
            switch update.Type {
            case "thought":
                return ReActStepMsg{...}
            case "action":
                return ReActStepMsg{...}
            // ...
            }
        }
        return nil
    },
)
```

## 验证

现在运行 TUI 并测试：
```
user: 当前文件夹有几个文件
```

应该能实时看到：
1. **Thought**: Agent 思考需要做什么
2. **Action**: 调用 glob 工具
3. **Observation**: 显示文件列表
4. **Response**: 返回文件计数

## 相关文件

- [internal/agent/agent.go](../internal/agent/agent.go) - ProgressUpdate 类型和 ChatWithToolsWithProgress 方法（第 38-49, 559-681 行）
- [internal/ui/handlers.go](../internal/ui/handlers.go) - 进度监听和转发（第 304-396 行）
