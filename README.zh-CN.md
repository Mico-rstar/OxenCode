<!-- 注意：本文件与 README.md 保持同步。更新时请同时更新两个文件。 -->

<div align="center">

# 🐂 OxenCode

**具有创新上下文管理能力的 AI 编程助手**

[English](README.md) | [简体中文](#readme)

</div>

---

## 项目概述

OxenCode 是一个用 Go 构建的学习型 AI 编程助手。它采用了独特的架构，将 Agent 执行环境与工具执行环境分离，并实现了创新的上下文管理系统，能够支持长程任务而不会因为超出模型上下文限制而失败。

> **注意：** 这是一个专注于 Agent 工程化最佳实践和创新上下文管理策略的学习项目。

## 核心特性

### 🏗️ Agent/Tool 环境分离

OxenCode 的核心创新是将 Agent 执行环境与工具执行环境分离。这一设计提供了：

- **增强安全性**：工具在隔离的环境中运行，防止意外副作用
- **提高可靠性**：工具失败不会导致 Agent 进程崩溃
- **更好的资源管理**：Agent 推理和工具执行分别有独立的资源限制

### ⏱️ 长程任务支持

与传统 AI 助手在上下文超出模型限制时失败不同，OxenCode 通过以下方式处理长时间任务：

- **分批上下文压缩**：异步、用户无感的上下文管理
- **上下文分级**：L0 > L1 > L2 压缩级别以获得最佳性能
- **Session 隔离**：任务上下文隔离以防止干扰
- **稳定前缀设计**：高缓存命中率以提高 token 使用效率

### 🔄 ReAct 循环实现

OxenCode 实现了 ReAct（推理 + 行动）模式用于迭代式问题解决：

- **思考 → 行动 → 观察** 循环处理复杂任务
- **流式处理**：实时显示 AI 推理和工具执行过程
- **错误恢复**：失败时自动重试和策略调整
- **LLM 推理显示**：对支持的模型显示思考过程

### 🛠️ 丰富的工具生态

用于常见开发任务的内置工具：

- **文件操作**：`Glob`、`Grep`、`Read`、`Write`、`Edit`
- **系统命令**：`Bash`，可配置超时时间
- **权限系统**：危险操作需要用户授权
- **智能验证**：工具执行前验证参数

### 🌐 多 Provider 支持

支持多个 LLM Provider：

- **Anthropic**：Claude（Sonnet、Haiku），支持扩展思考
- **OpenAI**：GPT-4、GPT-4o、o1 系列
- **Google**：Gemini 2.0 Flash、Gemini 1.5 Pro
- **Qwen**：Qwen-Max、Qwen-Plus、Qwen-Turbo
- **DeepSeek**：DeepSeek-Chat、DeepSeek-Coder、DeepSeek-Reasoner
- **GLM**：GLM-4 系列
- **Azure OpenAI**、**AWS Bedrock**、**OpenRouter** 等

## 快速开始

### 前置要求

- Go 1.25 或更高版本
- 您选择的 LLM Provider 的 API Key

### 安装

```bash
# 克隆仓库
git clone https://github.com/yourname/oxencode.git
cd oxencode

# 构建项目
go build -o oxencode ./cmd/oxencode

# （可选）安装到系统
go install ./cmd/oxencode
```

### 配置

1. 复制示例配置：

```bash
mkdir -p ~/.oxencode
cp config.example.toml ~/.oxencode/config.toml
```

2. 编辑 `~/.oxencode/config.toml` 配置您的设置：

```toml
# 选择您的 provider
provider = "anthropic"  # 或 "openai", "deepseek", "qwen" 等

# 设置模型
model = "claude-sonnet-4-5-20250514"

# 配置工作目录（可选）
work_dir = "."  # 当前目录

# 设置工具超时（可选）
tool_timeout = 120  # 秒
```

3. 将 API Key 设置为环境变量：

```bash
# 对于 Anthropic Claude
export ANTHROPIC_API_KEY="your-key-here"

# 对于 OpenAI
export OPENAI_API_KEY="your-key-here"

# 对于 DeepSeek
export DEEPSEEK_API_KEY="your-key-here"
```

### 运行 OxenCode

```bash
./oxencode
```

## 使用示例

### 基础对话

```
You: 这个目录下有什么文件？

OxenCode: [使用 Glob 工具列出文件]
```

### 代码分析

```
You: 找出所有包含 "error" 的 Go 文件，并给我看第一个。

OxenCode: [使用 Grep 查找文件，然后 Read 显示内容]
```

### 多步骤任务

```
You: 用 Go 创建一个简单的 HTTP 服务器，返回 "Hello, World"

OxenCode: [使用 Write 工具创建包含服务器代码的 main.go]
```

### 中断任务

随时按 `Esc` 键中断正在运行的任务。

## 文档

详细文档请参阅：

- [架构概述](docs/architecture.md) - 系统设计和数据流
- [ReAct 循环实现](docs/react-loop.md) - 推理循环如何工作
- [工具集成](docs/tool-integration.md) - 构建和注册工具
- [工具行为参考](docs/tools-behavior.md) - 可用工具及其用法
- [TUI 测试](docs/tui-testing.md) - 测试终端 UI

## 配置参考

完整配置选项请参阅 [config.example.toml](config.example.toml)。

主要配置区域：

- **Provider 选择**：从 10+ 个 LLM Provider 中选择
- **模型设置**：模型选择、温度、最大 token 数
- **扩展思考**：为支持的模型启用推理功能
- **工作目录**：工具操作位置
- **工具超时**：工具执行的安全限制

## 贡献

欢迎贡献！这是一个学习项目，欢迎您：

- 报告 bug
- 提出新功能建议
- 提交 pull request
- 改进文档
- 分享您的使用模式

贡献时，请：

1. Fork 仓库
2. 创建特性分支
3. 进行更改
4. 如适用，添加测试
5. 提交 pull request

## 架构亮点

OxenCode 采用分层架构：

```
┌─────────────────────────────────────┐
│     表示层 (TUI)                   │
│         Bubble Tea UI              │
└─────────────────────────────────────┘
                ↓
┌─────────────────────────────────────┐
│     应用层                         │
│    Chat / Tool / Auth 管理器       │
└─────────────────────────────────────┘
                ↓
┌─────────────────────────────────────┐
│     领域层                         │
│  Agent (ReAct) + 工具 + 权限       │
│         Fantasy SDK                │
└─────────────────────────────────────┘
                ↓
┌─────────────────────────────────────┐
│   基础设施层                       │
│  文件系统 / 配置 / 历史记录        │
└─────────────────────────────────────┘
```

## 路线图

- [ ] 增强的上下文压缩算法
- [ ] 并行工具执行
- [ ] 工具结果缓存
- [ ] 交互式调试模式
- [ ] 自定义工具插件系统
- [ ] UI 多语言支持

## 许可证

MIT 许可证 - 详见 LICENSE 文件

## 致谢

- [fantasy](https://github.com/charmbracelet/fantasy) - Go LLM 交互 SDK
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - 终端 UI 框架
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - 样式和格式化
- 所有让 AI 工具更易用的开源贡献者

---

<div align="center">

**用 ❤️ 构建，专注于学习 Agent 工程化**

[↑ 返回顶部](#-oxencode)

</div>
