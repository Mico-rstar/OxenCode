# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
# Build
go build -o oxencode ./cmd/oxencode

# Run
./oxencode

```

## Testing

```bash
# Run all tests
go test ./...

# Run tests for specific package
go test ./internal/agent/...
go test ./internal/tools/...

# Run tests with verbose output
go test -v ./internal/agent/... -run "TestAgentWithTools|TestExecuteTool"
```

## Architecture Overview

OxenCode is an AI programming assistant built with Go, featuring a layered architecture:

```
Presentation Layer (TUI)
    ↓
Application Layer (Chat/Tool/Auth Managers)
    ↓
Domain Layer (Agent ReAct Loop + Tools + Permissions)
    ↓
Infrastructure Layer (File System / Config / History)
```

### Core Components

- **cmd/oxencode/** - Main entry point, initializes Bubble Tea TUI
- **internal/ui/** - TUI implementation with Bubble Tea (model.go, view.go, handlers.go)
- **internal/agent/** - Agent core with ReAct loop, LLM integration via fantasy SDK
- **internal/tools/** - Tool system (Glob, Grep, Read, Write, Edit, Bash)
- **internal/prompt/** - Modular system prompt with INCLUDE directive support
- **pkg/config/** - Configuration management via Viper (config.toml)
- **pkg/api/** - API client wrappers
- **internal/message/** - Message types for conversation history and ReAct steps

### Key Patterns

1. **ReAct Loop**: Thought → Action → Observation cycle for iterative problem-solving
2. **Tool Registry**: Centralized tool registration and execution via `Environment` interface
3. **Environment Abstraction**: Tools operate through `Environment` interface for isolation
4. **Modular Prompts**: System prompt uses `{{INCLUDE:modules/xxx.md}}` for composition

## Configuration

Configuration loaded from `~/.oxencode/config.toml` with fallback to `config.example.toml`.

Key settings:
- `provider` - LLM provider (anthropic, openai, qwen, deepseek, etc.)
- `model` - Model name
- `work_dir` - Working directory for tool operations
- `tool_timeout` - Tool execution timeout in seconds
- `thinking_enabled` - Enable extended thinking for supported models

## Tool System

Available tools registered in Agent:
- **P0**: Glob, Grep, Read
- **P1**: Bash, Write, Edit

Tools are executed via `Agent.ExecuteTool()` with JSON Schema validation. Results are returned to LLM for ReAct iteration.

## Context Management (Planned)

The context management system (`internal/context/`) implements:
- **Session**: Isolates task contexts
- **Page**: Compresses messages with configurable schemas
- **L0/L1/L2**: Hierarchical compression levels (L0 = highest compression)
- **Async Compression**: Batch compression to prevent context limit issues

See `docs/context-manage-idea.md` for detailed design.


## 🚨 CRITICAL PRINCIPLES - READ CAREFULLY BEFORE MAKING ANY CHANGES 🚨

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