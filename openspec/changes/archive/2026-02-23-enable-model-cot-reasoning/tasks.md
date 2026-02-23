## 1. Agent 层实现

- [x] 1.1 在 `ChatWithToolsWithProgress` 方法中添加 `OnReasoningDelta` 回调
  - **文件**: `internal/agent/agent.go`
  - **位置**: 行 598 `fantasy.AgentStreamCall` 结构体
  - **变更**: 添加 `OnReasoningDelta: func(id, text string) error { ... }`
  - **实现**: 调用 `sendProgress("thought", text, "")` 发送推理内容
  - **验证**: 添加 `a.logger.Debug("Reasoning delta", "length", len(text))` 日志

- [x] 1.2 删除假的 thought 发送逻辑
  - **文件**: `internal/agent/agent.go`
  - **位置**: 行 584-585
  - **变更**: 注释或删除 `sendProgress("thought", "User is asking: "+userMessage, "")`
  - **理由**: 这个只是 echo 用户输入，不是真实的推理内容

- [x] 1.3 添加 thinking_budget 配置支持
  - **文件**: `pkg/config/config.go`
  - **变更**: 添加 `ThinkingBudget int`, `ThinkingEffort string`, `ThinkingEnabled bool` 配置字段
  - **实现**: 使用 `fantasy.WithProviderOptions` 传递 thinking 配置
  - **支持**: Anthropic (thinking_budget), OpenAI/OpenAICompat (thinking_effort), OpenRouter (extra_body)

- [x] 1.4 更新 config.example.toml
  - **文件**: `config.example.toml`
  - **变更**: 添加完整的 Extended Thinking 配置说明

---

## 2. 测试与验证

- [x] 2.1 手动测试推理内容捕获
  - **前提**: 使用支持 reasoning 的模型（如 DeepSeek-Reasoner）
  - **结果**: ✅ 推理内容正确显示，位于工具调用之前

- [x] 2.2 验证时间顺序正确
  - **结果**: ✅ thought (推理内容) → action (工具调用) → observation (工具结果)

- [x] 2.3 测试不支持的 Provider
  - **结果**: ✅ 工具调用正常工作，只是没有推理内容显示

---

## 3. TUI 层优化

- [x] 3.1 验证推理内容的视觉样式
  - **文件**: `internal/ui/styles.go`
  - **结果**: TUI 已有完整的样式支持（💭 图标 + ThoughtMsg 样式）

- [x] 3.2 确认 StateThinking 状态触发
  - **文件**: `internal/ui/handlers.go`
  - **结果**: 现有实现通过 ReAct 步骤提供视觉反馈

- [x] 3.3 修复推理内容流式渲染问题
  - **文件**: `internal/message/message.go`
  - **变更**: 添加 `AppendReActStep` 方法，累积相同类型的步骤内容
  - **文件**: `internal/ui/handlers.go`
  - **变更**: `thought` 类型使用 `AppendReActStep` 而不是 `AddReActStep`
  - **问题**: 之前每个字符创建一行，导致大量重复的 💭 图标
  - **解决**: 现在推理内容累积到单个步骤中

---

## 4. 文档更新

- [x] 4.1 更新 ReAct 循环文档
  - **文件**: `docs/react-loop.md`
  - **变更**: 添加 LLM Reasoning 支持章节，包括多 provider 配置说明

- [x] 4.2 更新工具集成文档（如需要）
  - **检查**: 文档未提及 `ProgressUpdate` 类型，无需更新
