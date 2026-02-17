# Agent 工具集成文档

本文档描述了 P0 工具如何集成到 Agent 中，以及如何使用这些工具。

---

## 目录

1. [概述](#概述)
2. [架构](#架构)
3. [工具注册](#工具注册)
4. [工具执行](#工具执行)
5. [API 参考](#api-参考)
6. [使用示例](#使用示例)
7. [测试](#测试)

---

## 概述

Agent 通过工具系统与文件系统交互。工具提供了安全的、受控的方式来执行文件操作，同时保持与 LLM 的集成。

### 集成的工具

当前集成了三个 P0 工具：

- **Glob** - 文件路径模式匹配
- **Grep** - 内容搜索
- **Read** - 文件读取

### 工具特性

- **环境隔离**: 所有工具通过 `Environment` 接口操作，支持本地和容器环境
- **参数验证**: 使用 JSON Schema 进行严格的参数验证
- **结构化日志**: 所有操作都有详细的日志记录
- **资源限制**: 内置资源限制，防止意外消耗过多资源

---

## 架构

### Agent 结构

```go
type Agent struct {
    agent         fantasy.Agent       // Fantasy LLM agent
    config        *config.Config      // 配置
    history       []message.Message   // 对话历史
    toolRegistry  *tools.Registry     // 工具注册表
    env           tools.Environment   // 执行环境
    logger        logger.Logger       // 日志记录器
}
```

### 组件关系

```
┌─────────────────────────────────────────────────┐
│                   Agent                          │
├─────────────────────────────────────────────────┤
│  • toolRegistry: 管理可用工具                    │
│  • env: 提供文件系统访问                         │
│  • logger: 记录操作日志                          │
└──────────────┬──────────────────────────────────┘
               │
       ┌───────┴────────┐
       │                │
       ▼                ▼
┌──────────────┐  ┌─────────────┐
│  Registry    │  │ Environment  │
│  - Glob      │  │  - ReadFile  │
│  - Grep      │  │  - WriteFile │
│  - Read      │  │  - ListFiles │
└──────────────┘  └─────────────┘
```

---

## 工具注册

### 初始化过程

在 `NewAgent` 创建时自动完成工具注册：

```go
// 创建执行环境
env, err := tools.NewLocalEnvironment(workDir, log)

// 创建工具注册表
registry := tools.NewRegistry(log)

// 注册 P0 工具
registry.Register(tools.NewGlobTool(env, log))
registry.Register(tools.NewGrepTool(env, log))
registry.Register(tools.NewReadTool(env, log))
```

### 工具注册表

工具注册表负责：

1. 管理可用工具
2. 提供工具查找
3. 生成工具 schemas（供 LLM 使用）
4. 执行工具调用

---

## 工具执行

### ExecuteTool 方法

```go
func (a *Agent) ExecuteTool(ctx context.Context, toolName string, input map[string]any) (string, error)
```

### 执行流程

```
1. 获取工具
   ├─ 工具不存在 → 返回错误
   └─ 工具存在 → 继续
        │
2. 参数验证
   ├─ 验证失败 → 返回错误
   └─ 验证成功 → 继续
        │
3. 执行工具
   ├─ 执行失败 → 返回错误
   └─ 执行成功 → 返回结果
```

### 错误处理

所有错误都包含描述性信息：

```go
// 工具不存在
"tool not found: <toolname>"

// 参数验证失败
"parameter validation failed: <details>"

// 工具执行失败
"tool execution failed: <details>"
```

---

## API 参考

### ExecuteTool

执行工具调用。

```go
func (a *Agent) ExecuteTool(ctx context.Context, toolName string, input map[string]any) (string, error)
```

**参数**:
- `ctx`: 上下文，用于取消和超时控制
- `toolName`: 工具名称（"glob", "grep", "read"）
- `input`: 工具输入参数

**返回**:
- `string`: 工具输出结果
- `error`: 错误信息（如果有）

### GetToolSchemas

获取所有工具的 schemas，用于传递给 LLM。

```go
func (a *Agent) GetToolSchemas() []map[string]any
```

**返回值**:
每个 schema 包含：
- `name`: 工具名称
- `description`: 工具描述
- `input_schema`: JSON Schema 参数定义

### GetEnvironment

获取执行环境。

```go
func (a *Agent) GetEnvironment() tools.Environment
```

### GetToolRegistry

获取工具注册表。

```go
func (a *Agent) GetToolRegistry() *tools.Registry
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

// 执行 Glob 工具
result, err := agent.ExecuteTool(ctx, "glob", map[string]any{
    "pattern": "*.go",
})
if err != nil {
    log.Printf("Error: %v", err)
}
fmt.Printf("Found files:\n%s\n", result)

// 执行 Grep 工具
result, err = agent.ExecuteTool(ctx, "grep", map[string]any{
    "pattern":      "func main",
    "file_pattern": "*.go",
})
fmt.Printf("Search results:\n%s\n", result)

// 执行 Read 工具
result, err = agent.ExecuteTool(ctx, "read", map[string]any{
    "file_path": "main.go",
    "limit":     20,
})
fmt.Printf("File content:\n%s\n", result)
```

### 获取工具 Schemas

```go
// 获取所有工具的 schemas
schemas := agent.GetToolSchemas()

for _, schema := range schemas {
    fmt.Printf("Tool: %s\n", schema["name"])
    fmt.Printf("Description: %s\n", schema["description"])
    fmt.Printf("Schema: %v\n\n", schema["input_schema"])
}
```

### 组合使用

```go
// 1. 查找所有 Go 文件
files, _ := agent.ExecuteTool(ctx, "glob", map[string]any{
    "pattern": "**/*.go",
})

// 2. 在这些文件中搜索 "TODO"
results, _ := agent.ExecuteTool(ctx, "grep", map[string]any{
    "pattern":      "TODO",
    "file_pattern": "*.go",
})

// 3. 读取包含 TODO 的文件
// (解析 results 获取文件路径，然后读取)
```

---

## 测试

### 测试覆盖

集成测试位于 [internal/agent/agent_test.go](../internal/agent/agent_test.go):

- `TestAgentWithTools` - 验证工具注册和 schemas
- `TestExecuteTool` - 测试工具执行

### 运行测试

```bash
# 运行所有 Agent 测试
go test ./internal/agent/...

# 只运行工具集成测试
go test ./internal/agent/... -run "TestAgentWithTools|TestExecuteTool"

# 查看详细输出
go test -v ./internal/agent/... -run "TestExecuteTool"
```

### 测试结果

```
=== RUN   TestAgentWithTools
=== RUN   TestAgentWithTools/Agent_has_tool_registry
=== RUN   TestAgentWithTools/Agent_has_environment
=== RUN   TestAgentWithTools/P0_tools_are_registered
=== RUN   TestAgentWithTools/GetToolSchemas_returns_schemas
--- PASS: TestAgentWithTools (0.00s)

=== RUN   TestExecuteTool
=== RUN   TestExecuteTool/Execute_glob_tool
=== RUN   TestExecuteTool/Execute_grep_tool
=== RUN   TestExecuteTool/Execute_read_tool
=== RUN   TestExecuteTool/Execute_non-existent_tool
=== RUN   TestExecuteTool/Execute_tool_with_invalid_parameters
--- PASS: TestExecuteTool (0.12s)

PASS
ok      github.com/yourname/oxencode/internal/agent    0.959s
```

---

## 工具 Schemas

### Glob

```json
{
  "name": "glob",
  "description": "使用通配符模式查找文件",
  "input_schema": {
    "type": "object",
    "properties": {
      "pattern": {
        "type": "string",
        "description": "文件匹配模式（支持 *, **, ? 等通配符）"
      },
      "path": {
        "type": "string",
        "description": "搜索路径，默认为当前目录"
      }
    },
    "required": ["pattern"]
  }
}
```

### Grep

```json
{
  "name": "grep",
  "description": "在文件中搜索匹配正则表达式的内容",
  "input_schema": {
    "type": "object",
    "properties": {
      "pattern": {
        "type": "string",
        "description": "正则表达式搜索模式"
      },
      "path": {
        "type": "string",
        "description": "搜索路径，默认为当前目录"
      },
      "file_pattern": {
        "type": "string",
        "description": "文件过滤模式（如 *.go）"
      },
      "ignore_case": {
        "type": "boolean",
        "description": "忽略大小写"
      }
    },
    "required": ["pattern"]
  }
}
```

### Read

```json
{
  "name": "read",
  "description": "读取文件内容",
  "input_schema": {
    "type": "object",
    "properties": {
      "file_path": {
        "type": "string",
        "description": "要读取的文件路径"
      },
      "offset": {
        "type": "integer",
        "description": "起始行号（从 1 开始），默认为 1"
      },
      "limit": {
        "type": "integer",
        "description": "读取行数，默认读取全部"
      }
    },
    "required": ["file_path"]
  }
}
```

---

## 下一步

### Phase 3.3: ReAct 循环集成

下一步将是实现完整的 ReAct 循环：

1. **LLM 工具调用识别**: 解析 LLM 返回的工具调用请求
2. **工具执行器**: 异步执行工具并返回结果
3. **ReAct 消息流**: 将工具调用和结果集成到消息历史
4. **TUI 集成**: 在用户界面中显示工具调用过程

### Phase 3.4: P1 工具

添加更多工具：

- **Bash** - 执行 shell 命令
- **Write** - 写入文件
- **Edit** - 编辑文件

---

## 相关文档

- [工具系统设计文档](wip/tool.md) - 架构设计和实现细节
- [工具行为文档](tools-behavior.md) - 工具使用参考
- [消息系统文档](../internal/message/message.go) - 消息和 ReAct 循环定义

---

**最后更新**: 2026-02-17
**版本**: v0.1.0
