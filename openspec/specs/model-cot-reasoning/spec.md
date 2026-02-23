# Model Chain of Thought Reasoning Support

## Overview

OxenCode supports capturing and displaying LLM Chain of Thought (CoT) reasoning content, allowing users to see the model's thinking process before it executes tool calls.

## Requirements

### Requirement: Capture model reasoning content

系统必须能够捕获 LLM 生成的 Chain of Thought 推理内容，并通过进度更新机制传递给 TUI 层展示。

#### Scenario: LLM generates reasoning before tool call
- **WHEN** LLM 在执行工具调用前生成推理内容（reasoning/thinking）
- **THEN** Agent 应通过 `OnReasoningDelta` 回调捕获这些内容
- **AND** 每次收到 reasoning delta 时，通过 `ProgressUpdate` channel 发送 `thought` 类型的更新
- **AND** 推理内容应在 `action` 类型更新（工具调用）之前发送

#### Scenario: LLM generates reasoning without tool call
- **WHEN** LLM 生成推理内容但不需要调用工具
- **THEN** 推理内容仍应被捕获并发送到 TUI
- **AND** 最终应发送 `done` 类型的更新包含完整的响应内容

#### Scenario: LLM does not support reasoning
- **WHEN** 使用的 provider 不支持推理内容（如某些 OpenAI 模型）
- **THEN** 系统应正常工作，不发送 `thought` 类型更新
- **AND** 不影响现有的工具调用和响应流程

---

### Requirement: Stream reasoning content in real-time

推理内容必须以流式方式实时发送，而不是等待完整推理结束后才发送。

#### Scenario: Reasoning content arrives in chunks
- **WHEN** `OnReasoningDelta` 回调被多次调用，每次传入推理内容的一部分
- **THEN** 每次调用都应立即发送一个 `ProgressUpdate`，包含当前增量内容
- **AND** 增量内容应追加到而不是替换之前的推理内容

#### Scenario: Reasoning starts and ends
- **WHEN** `OnReasoningStart` 回调被调用
- **THEN** 可以选择发送一个标记推理开始的特殊更新（可选）
- **WHEN** `OnReasoningEnd` 回调被调用
- **THEN** 可以选择发送一个标记推理结束的特殊更新（可选）

---

### Requirement: Display reasoning content in TUI

TUI 必须能够接收并展示推理内容，使其在视觉上与最终回答区分开。

#### Scenario: Thought progress update is received
- **WHEN** TUI 收到 `Type: "thought"` 的 `ProgressUpdate`
- **THEN** 应将应用状态设置为 `StateThinking`
- **AND** 推理内容应以特殊样式显示（如灰色、斜体或可折叠区块）
- **AND** 推理内容应与工具调用（`action`）和最终回答（`done`）清晰区分

#### Scenario: Reasoning followed by tool call
- **WHEN** 推理内容展示后，用户看到工具调用
- **THEN** 用户应能理解：模型先思考，然后决定执行这个工具
- **AND** 时间顺序应为：reasoning → action → observation → [repeat or done]

---

### Requirement: Backward compatibility

新功能必须与现有代码向后兼容，不破坏现有的工具调用流程。

#### Scenario: Existing ChatWithTools behavior preserved
- **WHEN** 现有代码调用 `ChatWithTools` 方法（不带进度更新）
- **THEN** 该方法的行为应保持不变
- **AND** 不影响现有的 ReAct loop 逻辑

#### Scenario: Existing ProgressUpdate consumers
- **WHEN** TUI 处理 `ProgressUpdate` 时
- **THEN** 新增的 `thought` 类型应被正确处理或忽略（取决于实现）
- **AND** 不应导致 TUI 崩溃或状态混乱

---

## Provider Compatibility

| Provider | Reasoning Support | Configuration Method |
|----------|-------------------|---------------------|
| Anthropic (Claude 3.7+) | ✅ Supported | `thinking_budget` (token number) |
| OpenAI (o1 series) | ✅ Supported | `thinking_effort` (minimal/low/medium/high) |
| OpenRouter | ✅ Supported | `thinking_enabled` or `thinking_effort` |
| Qwen (via OpenRouter) | ✅ Supported | `thinking_enabled` with `enable_thinking` |
| Qwen (direct) | ⚠️ Partial | Requires OpenRouter for full support |
| DeepSeek, GLM | ⚠️ Model-dependent | `thinking_effort` |
| Google Gemini | ❌ Not supported | Callback won't trigger |

## Configuration Examples

```toml
# Anthropic Claude
thinking_enabled = true
thinking_budget = 20000

# OpenAI (o1 series)
thinking_enabled = true
thinking_effort = "medium"

# OpenRouter (recommended for Qwen, etc.)
thinking_enabled = true
```

---

**Version:** 1.0
**Last Updated:** 2026-02-23
