# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

OxenCode 是一个学习型的 AI 编程助手项目，专注于实践 Agent 工程化的核心技术。

**核心特性**:
- 流式输出 - 打字机效果，实时显示 AI 思考过程
- 工具调用 - Glob/Grep/Read/Write/Edit/Bash 六大工具
- 权限系统 - 危险操作确认，支持持久授权
- 多轮对话 - 完整的上下文记忆
- 记忆系统 - 长期记忆存储与检索

**技术栈**: Go + Bubble Tea (TUI) + fantasy SDK (LLM)

## 数据目录

**~/.oxencode/** 是统一的运行时数据目录：

```
~/.oxencode/
├── config.toml      # 配置文件
├── memory/          # 记忆数据
│   ├── experience/  # 经验规则 (遇到X应该Y)
│   ├── knowledge/   # 事实性知识
│   ├── notes/       # 会话摘要
│   ├── histories/   # 原始对话轨迹
│   └── inner/       # 自我/用户认知 (自动装载)
├── chromadb/        # Chroma 向量数据库
└── metadata.db      # 元数据数据库
```

## 架构

详见 [docs/architecture.md](docs/architecture.md)

```
Presentation Layer (TUI)
    ↓
Application Layer (Chat/Tool/Auth Managers)
    ↓
Domain Layer (Agent ReAct Loop + Tools + Permissions)
    ↓
Infrastructure Layer (File System / Config / History)
```

## 目录结构

```
cmd/oxencode/     # 入口点
internal/
├── agent/        # ReAct 循环
├── tools/        # 工具系统
├── ui/           # TUI (Bubble Tea)
├── context/      # 上下文管理
├── message/      # 消息类型
└── cli/          # CLI 命令
pkg/
├── config/       # 配置管理
├── prompt/       # 模块化提示词
├── memory/       # 记忆服务客户端
├── api/          # API 客户端
├── logger/       # 日志
└── paths/        # 路径管理
memsvc/           # Python 记忆服务 (FastAPI + Chroma)
```

## 核心组件

### Agent (ReAct Loop)

详见 [docs/react-loop.md](docs/react-loop.md)

Thought → Action → Observation 循环，支持：
- 多轮工具调用迭代
- LLM Reasoning 展示 (extended thinking)
- 最大迭代次数限制

### 工具系统

详见 [docs/tool.md](docs/tool.md)

**P0 工具**: Glob, Grep, Read
**P1 工具**: Bash, Write, Edit
**记忆工具**: trigger_memory, search_memory, load_memory

工具通过 `Environment` 接口执行，支持本地环境隔离。

### 提示词系统

模块化设计，支持 `{{INCLUDE:modules/xxx.md}}` 指令：

```
pkg/prompt/prompts/
├── main_prompt.md      # 主提示词
└── modules/
    ├── core.md         # 核心规则
    ├── base_tools.md   # 基础工具说明
    └── memory_tools.md # 记忆工具说明
```

### 记忆系统

详见 [docs/memory/design.md](docs/memory/design.md)

**架构**: Go 端 (HTTP Client) + Python 端 (FastAPI + Chroma)

**核心接口**:
- `POST /trigger_memory` - 快速判断是否有相关记忆
- `POST /search_memory` - RAG 检索 + rerank
- `POST /commit_session` - 提交会话，异步压缩和整理

**工作流程**:
1. Session 开始 → inner 目录自动装载到上下文
2. 需要时调用 search_memory 检索相关记忆
3. Session 结束 → commit_session 触发异步任务
4. 后台任务：压缩 notes → 多 Agent 整理 → re_embed

## 配置

详见 [config.example.toml](config.example.toml)

关键配置项：
- `provider` / `model` - LLM 提供商和模型
- `work_dir` - 工具操作工作目录
- `memory_enabled` - 是否启用记忆服务
- `memory_service_url` - 记忆服务地址

## Build & Run

```bash
# Build
go build -o oxencode ./cmd/oxencode

# Run
./oxencode

# 启动记忆服务 (需要单独启动)
cd memsvc && uv run python -m memsvc
```

## Testing

```bash
# Run all tests
go test ./...

# Run tests for specific package
go test ./internal/agent/...
go test ./internal/tools/...
```

## CRITICAL PRINCIPLES - READ CAREFULLY BEFORE MAKING ANY CHANGES

These principles are **NON-NEGOTIABLE**. Violating any of these will result in unacceptable solutions.

1. **ALWAYS reuse existing components first.**
   - Check `pkg/` directory for reusable packages (logger, config, prompt etc.)
   - Never duplicate existing functionality without a compelling reason

2. **ALWAYS solve the root cause directly.**
   - Do NOT use workarounds or hacks to bypass problems
   - If you encounter an issue, address it at its source

3. **NEVER compromise testing for higher pass rates.**
   - Do NOT intentionally skip testing critical environments
   - Do NOT use clever designs to artificially pass tests while ignoring real issues
   - Tests must reflect actual production behavior

4. **STOP and ask for human help when blocked.**
   - If you cannot solve a problem, encounter contradictions, or have doubts
   - Do NOT proceed with workarounds or assumptions
   - Explicitly state the issue and wait for human guidance

5. **USE PRODUCTION-GRADE LIBRARIES.**
   - Before implementing new functionality, check for mature, production-ready libraries
   - If uncertain about whether a suitable library exists, ASK the human for a decision
   - Do NOT reinvent the wheel unless explicitly directed to do so

6. **ASK human for more context when anything not sure**

7. **Never use mocks as a fallback strategy.**
   - mock is a kind of workaround, it will delay the real errors to the production environment
   - panic > mock

## 重要提醒
- 有tui和cli两种交互模式，但现在开发者主要维护的是cli模式，斜杠命令只在cli模式支持