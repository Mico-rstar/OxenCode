# TUI 集成测试文档

本文档说明如何测试 OxenCode 的 TUI 集成，验证 P0 工具（Glob、Grep、Read）的 ReAct 循环是否正常工作。

---

## 目录

1. [快速开始](#快速开始)
2. [配置](#配置)
3. [测试场景](#测试场景)
4. [预期行为](#预期行为)
5. [ReAct 步骤显示](#react-步骤显示)
6. [故障排查](#故障排查)

---

## 快速开始

### 1. 构建应用

```bash
cd /home/rene/projs/OxenCode
go build ./cmd/oxencode
```

### 2. 配置 API Key

设置环境变量（根据你的 provider 选择）：

```bash
# Anthropic (Claude)
export ANTHROPIC_API_KEY="your-anthropic-key"

# OpenAI
export OPENAI_API_KEY="your-openai-key"

# Qwen (通义千问)
export DASHSCOPE_API_KEY="your-qwen-key"

# DeepSeek
export DEEPSEEK_API_KEY="your-deepseek-key"
```

### 3. 运行应用

```bash
./oxencode
```

---

## 配置

### 创建配置文件（可选）

创建 `~/.oxencode/config.toml`:

```toml
provider = "anthropic"  # 或 "openai", "qwen", "deepseek" 等
model = "claude-sonnet-4-5-20250514"
max_tokens = 8192
temperature = 0.7
```

---

## 测试场景

### 场景 1: 简单对话（不使用工具）

**输入**:
```
你好，请介绍一下你自己
```

**预期行为**:
- 直接返回 AI 回复
- 不显示任何 ReAct 步骤
- 显示用户消息和助手消息

### 场景 2: 使用 Glob 工具

**输入**:
```
请列出当前目录下所有的 Go 文件
```

**预期行为**:
1. 显示 "💭 Processing user request" (thought)
2. 显示 "🔧 glob" (action)
3. 显示工具结果（文件列表）
4. 显示最终答案

**预期输出示例**:
```
🧒 User: 请列出当前目录下所有的 Go 文件

🤖 Assistant ⏳
├─💭 Processing user request: 请列出当前目录下所有的 Go 文件
├─🔧 glob ⏳
│   └─ Result: internal/agent/agent.go
              internal/agent/agent_test.go
              internal/message/message.go
              ...
Found 15 Go files in the project. The main files are...
```

### 场景 3: 使用 Grep 工具

**输入**:
```
搜索所有包含 "package" 关键词的 Go 文件
```

**预期行为**:
1. 显示 thought
2. 显示 action（grep 工具）
3. 显示搜索结果
4. 显示总结

**预期输出示例**:
```
🧒 User: 搜索所有包含 "package" 关键词的 Go 文件

🤖 Assistant ⏳
├─💭 Processing user request...
├─🔧 grep ⏳
│   └─ Result: internal/agent/agent.go:1:package agent
              internal/tools/glob.go:1:package tools
              ...
Found "package" keyword in 20 files. These are the package declarations...
```

### 场景 4: 使用 Read 工具

**输入**:
```
请读取 main.go 文件的前 20 行
```

**预期行为**:
1. 显示 thought
2. 显示 action（read 工具）
3. 显示文件内容（带行号）
4. 显示可能的总结

**预期输出示例**:
```
🧒 User: 请读取 main.go 文件的前 20 行

🤖 Assistant ⏳
├─💭 Processing user request...
├─🔧 read ⏳
│   └─ Result:     1→package main
                  2→
                  3→import (
                  4→        "fmt"
                  5→)
                  ...
Here are the first 20 lines of main.go...
```

### 场景 5: 复杂任务（多工具）

**输入**:
```
请找出所有包含 "TODO" 注释的 Go 文件，并读取第一个文件的内容
```

**预期行为**:
1. 显示 thought
2. Glob 查找所有 Go 文件
3. Grep 搜索 "TODO"
4. Read 读取第一个文件
5. 显示最终答案

**预期输出示例**:
```
🧒 User: 请找出所有包含 "TODO" 注释的 Go 文件...

🤖 Assistant ⏳
├─💭 Processing user request...
├─🔧 glob ⏳
│   └─ Result: (list of .go files)
├─💭 Now searching for TODO...
├─🔧 grep ⏳
│   └─ Result: internal/agent/agent.go:42:// TODO: add error handling
              internal/ui/model.go:15:// TODO: implement feature
├─💭 Reading first file...
├─🔧 read ⏳
│   └─ Result: (file content)
└─💭 All tools completed

Found 2 TODO comments. Reading the first file from agent.go...
```

---

## ReAct 步骤显示

### 符号说明

| 符号 | 含义 |
|------|------|
| 🧒 | 用户消息 |
| 🤖 | AI 助手 |
| 💭 | 思考 (thought) |
| 🔧 | 工具调用 (action) |
| ✅ | 完成 |
| ⏳ | 进行中 |
| ❌ | 错误 |

### 状态图标

| 状态 | 图标 |
|------|------|
| Pending | ○ |
| Streaming | ⏳ |
| Completed | ✅ |
| Error | ❌ |
| Cancelled | ⊘ |

### 树状结构

```
├─ 步骤 1
├─ 步骤 2
│   ├─ 子步骤 2.1
│   └─ 子步骤 2.2
└─ 步骤 3 (最后一步)
```

---

## 预期行为详细说明

### 1. 消息发送流程

```
用户输入 → 按 Enter → 清空输入框 → 显示 "thinking" → 执行工具 → 显示结果
```

### 2. 状态转换

```
Idle → Thinking → Streaming → ExecutingTool → Idle
```

### 3. 工具执行过程

1. **Thinking 状态**: 显示 "⏳" 图标
2. **执行工具**: 显示 "🔧 toolname" + "⏳"
3. **工具完成**: 显示 "✅" + 结果预览
4. **继续循环**: 如果需要更多工具调用
5. **最终响应**: 显示完整文本

### 4. 消息结构

每条助手消息包含：
- 消息头部（图标、角色、时间戳、状态）
- ReAct 循环步骤（如果有）
- 最终响应内容

---

## 故障排查

### 问题 1: 工具不执行

**可能原因**:
- API key 未设置或无效
- Provider 配置错误
- 工具未注册

**解决方法**:
1. 检查环境变量是否设置
2. 查看日志输出
3. 确认配置文件正确

### 问题 2: ReAct 步骤不显示

**可能原因**:
- AI 没有调用工具
- 消息渲染逻辑错误

**解决方法**:
1. 尝试明确的工具请求，如 "列出所有 Go 文件"
2. 检查日志中的 ReAct 循环信息
3. 确认 `len(msg.ReActLoop) > 0`

### 问题 3: 程序卡住

**可能原因**:
- API 请求超时
- 工具执行阻塞
- 上下文未取消

**解决方法**:
1. 按 `Ctrl+C` 或 `Esc` 中断
2. 检查网络连接
3. 查看日志中的错误信息

### 问题 4: 显示错误消息

**常见错误**:

```
"agent not available"
```
**原因**: Agent 初始化失败
**解决**: 检查配置文件和 API key

```
"API key not found"
```
**原因**: 环境变量未设置
**解决**: 导出对应的环境变量

```
"tool not found: xxx"
```
**原因**: 工具未注册或名称错误
**解决**: 检查工具注册代码

---

## 测试检查清单

### 基本功能

- [ ] 程序启动正常
- [ ] 可以输入消息
- [ ] 可以发送消息（按 Enter）
- [ ] 状态栏显示正确

### 工具功能

- [ ] Glob 工具可以列出文件
- [ ] Grep 工具可以搜索内容
- [ ] Read 工具可以读取文件
- [ ] 工具结果显示正确

### ReAct 循环

- [ ] 显示 thought 步骤
- [ ] 显示 action 步骤
- [ ] 显示工具结果
- [ ] 支持多轮工具调用
- [ ] 正确显示最终答案

### UI 显示

- [ ] 图标显示正确
- [ ] 状态图标更新
- [ ] ReAct 树状结构正确
- [ ] 消息格式美观

### 错误处理

- [ ] 工具失败时显示错误
- [ ] 可以中断长时间运行的任务
- [ ] 错误消息清晰有用

---

## 日志调试

### 启用详细日志

```bash
# 设置日志级别
export LOG_LEVEL=debug

# 运行程序
./oxencode
```

### 日志位置

日志输出到 stdout，包含：
- ReAct 循环迭代信息
- 工具执行详情
- 错误堆栈跟踪

### 关键日志信息

```
{"level":"INFO","msg":"Tools registered","count":3}
{"level":"INFO","msg":"Starting ChatWithTools"}
{"level":"DEBUG","msg":"ReAct iteration","iteration":1}
{"level":"INFO","msg":"Tool call requested","tool":"glob"}
{"level":"INFO","msg":"Tool executed successfully","tool":"glob"}
```

---

## 性能观察

### 响应时间预期

- **简单对话**: 1-3 秒
- **单个工具**: 2-5 秒
- **多工具任务**: 5-15 秒

### 资源使用

- **内存**: ~50-100MB
- **CPU**: 空闲时 < 1%，执行时 5-20%
- **网络**: 每次 LLM 调用 ~1-5KB 数据

---

## 下一步

测试完成后，可以：

1. **实现 P1 工具**: Bash、Write、Edit
2. **优化显示**: 更丰富的工具结果展示
3. **添加权限控制**: 询问用户是否执行危险操作
4. **保存历史**: 将对话历史保存到文件
5. **流式响应**: 显示实时响应进度

---

## 相关文档

- [工具集成文档](tool-integration.md)
- [ReAct 循环文档](react-loop.md)
- [工具行为文档](tools-behavior.md)

---

**最后更新**: 2026-02-17
**版本**: v0.1.0
