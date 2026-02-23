## Why

OxenCode 目前缺乏类似 Claude Code 的 "think → action" 可视化体验。当前的 ReAct loop 实现虽然能够执行工具调用，但无法展示模型在决策过程中的思考内容（Chain of Thought reasoning）。

通过探索发现：
1. 底层 fantasy 库 (v0.8.1) 已支持 `OnReasoningDelta` 回调来捕获 LLM 的推理内容
2. OxenCode 的 `ChatWithToolsWithProgress` 方法只注册了工具相关回调，未注册 reasoning 回调
3. 当前所谓的 "thought" 进度更新只是简单 echo 用户输入，并非真实的模型推理

让用户看到模型的思考过程能带来更好的体验：
- **透明度**：用户能理解模型为什么选择某个工具
- **信任度**：展示推理过程让 AI 的行为更可预测
- **调试**：开发者可以更容易诊断模型决策问题

## What Changes

实现模型推理内容（Chain of Thought）的捕获、传输和 TUI 实时展示。

具体变更：

1. **Agent 层（`internal/agent/agent.go`）**
   - 在 `ChatWithToolsWithProgress` 方法中注册 `OnReasoningStart`、`OnReasoningDelta`、`OnReasoningEnd` 回调
   - 通过 `ProgressUpdate` channel 发送推理内容
   - 确保推理内容在工具调用之前被发送

2. **ProgressUpdate 类型扩展**
   - 当前已有的类型：`thought`, `action`, `observation`, `content`, `error`, `done`
   - 复用 `thought` 类型，但这次发送真实的 LLM 推理内容

3. **TUI 层（`internal/ui/`）**
   - 验证 `StateThinking` 状态能正确展示推理内容
   - 确保推理内容以不同样式（如可折叠或特殊颜色）展示，与最终回答区分


## Capabilities

### New Capabilities

- `model-cot-reasoning`: 捕获和展示 LLM 的 Chain of Thought 推理内容，在 TUI 中以流式方式展示模型在执行工具调用前的思考过程

### Modified Capabilities

*(无现有 spec 需要修改，这是纯新增功能)*

## Impact

**受影响的代码模块**：
- `internal/agent/agent.go` - 核心变更点
- `internal/agent/agent_test.go` - 可能需要添加测试
- `internal/ui/handlers.go` - 处理 `thought` 类型进度更新
- `internal/ui/view.go` - 渲染推理内容的样式
- `internal/ui/model.go` - 已有 `StateThinking` 状态

**依赖关系**：
- fantasy 库 v0.8.1 (已提供 reasoning 回调接口)
- 无新增外部依赖

**API 变更**：
- `ProgressUpdate` 类型保持向后兼容
- `ChatWithToolsWithProgress` 方法签名不变

**风险**：
- 低风险：主要是添加新回调，不影响现有工具调用流程
- 需要验证不同 provider（Anthropic、OpenAI 等）对 reasoning 内容的支持程度
