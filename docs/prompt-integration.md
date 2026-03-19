# OxenCode 系统提示词动态加载集成

## 概述

为 OxenCode Agent 实现了灵活的系统提示词动态加载功能，支持从配置文件指定提示词目录，运行时重新加载，以及模块化的提示词管理。

## 实现内容

### 1. 配置扩展 ([pkg/config/config.go](pkg/config/config.go))

添加了 `PromptDir` 配置项：

```go
type Config struct {
    // ... 现有配置 ...
    PromptDir string `mapstructure:"prompt_dir"` // 系统提示词目录，默认为 internal/prompt
}
```

**配置文件** (`~/.oxencode/config.toml`):
```toml
prompt_dir = "internal/prompt"  # 默认值
```

### 2. Agent 集成 ([internal/agent/agent.go](internal/agent/agent.go))

#### 自动加载
Agent 初始化时自动从配置的目录加载系统提示词：

```go
func NewAgent(cfg *config.Config) (*Agent, error) {
    // ... 现有代码 ...
    systemPrompt := loadSystemPrompt(cfg, log)
    return &Agent{
        history: []message.Message{
            message.NewMessage(message.RoleSystem, systemPrompt),
        },
        // ...
    }
}
```

#### 动态重载
提供运行时重新加载方法：

```go
func (a *Agent) ReloadSystemPrompt() error {
    newPrompt := loadSystemPrompt(a.config, a.logger)
    a.SetSystemPrompt(newPrompt)
    a.logger.Info("System prompt reloaded")
    return nil
}
```

#### 降级机制
如果提示词加载失败，自动使用默认提示词：
```go
func loadSystemPrompt(cfg *config.Config, log logger.Logger) string {
    loader := prompt.NewLoader(promptDir)
    systemPrompt, err := loader.Load()
    if err != nil {
        log.Warn("Failed to load system prompt from file, using default")
        return getDefaultSystemPrompt()
    }
    return systemPrompt
}
```

### 3. 系统提示词内容 ([internal/prompt/](internal/prompt/))

#### 结构
```
internal/prompt/
├── main_prompt.md       # 主提示词（使用 INCLUDE 引用模块）
├── modules/
│   ├── core.md         # 核心原则：交互风格、边界、ReAct 工作流
│   └── tools.md        # 工具使用指导（Glob, Grep, Read, Bash）
├── loader.go           # 提示词加载器（支持 {{INCLUDE}} 指令）
└── examples/
    └── loader/
        └── main.go     # 示例程序
```

#### 特点
- **Token 效率**: ~1020 tokens，简洁高效
- **模块化**: 可重用的核心原则和工具指导
- **ReAct 导向**: 明确的 Thought-Action-Observation 循环指导
- **工具聚焦**: 详细的 Glob/Grep/Read/Bash 使用说明

## 使用方式

### 方式 1: 自动加载（推荐）

```go
cfg, _ := config.Load()
agent, _ := agent.NewAgent(cfg)  // 自动加载系统提示词
```

### 方式 2: 运行时重载

```go
agent.ReloadSystemPrompt()  // 重新加载提示词
```

### 方式 3: 手动加载

```go
loader := prompt.NewLoader("custom/prompt/dir")
systemPrompt, _ := loader.Load()
agent.SetSystemPrompt(systemPrompt)
```

## 测试验证

```bash
# 测试提示词加载器
go test ./internal/prompt/... -v

# 测试 Agent 集成
go test ./internal/agent/... -v

# 运行示例程序
go run examples/agent_with_prompt/main.go
```

## 实际效果

```
=== System Prompt Loaded ===
Source: internal/prompt
Length: 4082 characters (~1020 tokens)

Agent Response:
"I have the following tools available:
1. **Glob** – Find files by pattern
2. **Grep** – Search file contents
3. **Read** – Read the contents of a file
4. **Bash** – Execute shell commands
"
```

## 优势

1. **灵活性**: 无需重新编译即可修改系统提示词
2. **模块化**: 易于维护和扩展提示词内容
3. **降级安全**: 加载失败时使用默认提示词
4. **开发友好**: 便于测试不同提示词变体
5. **配置驱动**: 通过配置文件自定义提示词目录

## 后续改进

1. 添加提示词版本管理
2. 支持多种提示词变体（如 code-review、debug 模式）
3. 添加提示词 A/B 测试功能
4. 支持远程提示词加载
5. 添加提示词性能监控

## 相关文件

- [pkg/config/config.go](pkg/config/config.go) - 配置扩展
- [internal/agent/agent.go](internal/agent/agent.go) - Agent 集成
- [internal/prompt/main_prompt.md](internal/prompt/main_prompt.md) - 主提示词
- [internal/prompt/loader.go](internal/prompt/loader.go) - 加载器实现
- [examples/agent_with_prompt/main.go](examples/agent_with_prompt/main.go) - 示例程序
