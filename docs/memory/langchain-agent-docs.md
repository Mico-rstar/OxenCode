# LangChain Agent 使用文档

> 基于 LangChain 官方文档整理
> 最后更新：2026-03-20

---

## 目录

1. [概述](#1-概述)
2. [快速开始](#2-快速开始)
3. [核心组件](#3-核心组件)
4. [调用 Agent](#4-调用-agent)
5. [高级概念](#5-高级概念)
6. [Middleware](#6-middleware)
7. [框架对比](#7-框架对比-langchain-vs-langgraph-vs-deep-agents)
8. [最佳实践](#8-最佳实践)

---

## 1. 概述

LangChain Agent 将语言模型与工具结合，创建能够推理任务、决策使用哪些工具并迭代解决问题的系统。

### 核心特点

- 使用 `create_agent` 提供生产就绪的 agent 实现
- Agent 在循环中运行工具以达成目标
- 当模型发出最终输出或达到迭代限制时停止
- 基于 LangGraph 构建图运行时

### ReAct 模式

Agent 遵循 **"Reasoning + Acting"** 模式，交替进行推理步骤和目标工具调用，直到提供最终答案。

```
输入 → 模型 → 动作 (action) → 工具执行 → 观察 (observation) → 模型 → ... → 最终输出
```

---

## 2. 快速开始

### 安装

```bash
pip install -qU langchain "langchain[anthropic]"
```

### 基础示例

```python
from langchain.agents import create_agent

def get_weather(city: str) -> str:
    """Get weather for a given city."""
    return f"It's always sunny in {city}!"

agent = create_agent(
    model="claude-sonnet-4-6",
    tools=[get_weather],
    system_prompt="You are a helpful assistant"
)

# 运行 agent
result = agent.invoke({
    "messages": [{"role": "user", "content": "what is the weather in sf"}]
})
```

---

## 3. 核心组件

### 3.1 模型 (Model)

模型是 agent 的推理引擎，支持静态和动态模型选择。

#### 静态模型

```python
# 方式 1: 使用模型标识符（自动推断 provider）
agent = create_agent("openai:gpt-5", tools=tools)
# 或简写：agent = create_agent("gpt-5", tools=tools)

# 方式 2: 使用模型实例（更多配置控制）
from langchain_openai import ChatOpenAI

model = ChatOpenAI(
    model="gpt-5",
    temperature=0.1,
    max_tokens=1000,
    timeout=30
)
agent = create_agent(model, tools=tools)
```

**模型标识符格式：** `provider:model_name`，如 `openai:gpt-5`、`anthropic:claude-sonnet-4-6`、`google:gemini-2.5-pro`

#### 动态模型

动态模型在运行时根据当前状态和上下文选择，适用于复杂的路由逻辑和成本优化。

```python
from langchain_openai import ChatOpenAI
from langchain.agents import create_agent
from langchain.agents.middleware import wrap_model_call, ModelRequest, ModelResponse

basic_model = ChatOpenAI(model="gpt-4.1-mini")
advanced_model = ChatOpenAI(model="gpt-4.1")

@wrap_model_call
def dynamic_model_selection(request: ModelRequest, handler) -> ModelResponse:
    """根据对话复杂度选择模型"""
    message_count = len(request.state["messages"])

    if message_count > 10:
        # 长对话使用高级模型
        model = advanced_model
    else:
        model = basic_model

    return handler(request.override(model=model))

agent = create_agent(
    model=basic_model,  # 默认模型
    tools=tools,
    middleware=[dynamic_model_selection]
)
```

> **注意：** 使用结构化输出时，不支持预绑定模型（已调用 bind_tools 的模型）。

### 3.2 工具 (Tools)

工具让 agent 能够执行动作。Agent 支持：

- 单次提示触发多次工具调用
- 并行工具调用
- 基于先前结果动态选择工具
- 工具重试逻辑和错误处理
- 工具调用间的状态持久化

#### 静态工具

```python
from langchain.tools import tool
from langchain.agents import create_agent

@tool
def search(query: str) -> str:
    """Search for information."""
    return f"Results for: {query}"

@tool
def get_weather(location: str, unit: str = "celsius") -> str:
    """Get weather information for a location."""
    return f"Weather in {location}: Sunny, 72°F"

# 工具可以是普通函数或协程
agent = create_agent(model, tools=[search, get_weather])
```

使用 `@tool` 装饰器可以自定义工具名称、描述、参数 schema 等属性。

#### 动态工具

动态工具在运行时修改可用工具集，适用于基于认证状态、用户权限、功能标志或对话阶段适配工具集。

```python
from langchain.agents.middleware import wrap_model_call, ModelRequest, ModelResponse
from typing import Callable

@wrap_model_call
def state_based_tools(
    request: ModelRequest,
    handler: Callable[[ModelRequest], ModelResponse
) -> ModelResponse:
    """根据对话状态过滤工具"""
    state = request.state
    is_authenticated = state.get("authenticated", False)
    message_count = len(state["messages"])

    # 认证前只允许使用公开工具
    if not is_authenticated:
        tools = [t for t in request.tools if t.name.startswith("public_")]
        request = request.override(tools=tools)
    elif message_count < 5:
        # 对话早期限制工具
        tools = [t for t in request.tools if t.name != "advanced_search"]
        request = request.override(tools=tools)

    return handler(request)

agent = create_agent(
    model="gpt-4.1",
    tools=[public_search, private_search, advanced_search],
    middleware=[state_based_tools]
)
```

**适用场景：**
- 所有可能工具在编译/启动时已知
- 需要基于权限、功能标志或对话状态过滤
- 工具是静态的，但可用性是动态的

#### 工具错误处理

```python
from langchain.agents import create_agent
from langchain.agents.middleware import wrap_tool_call
from langchain.messages import ToolMessage

@wrap_tool_call
def handle_tool_errors(request, handler):
    """用自定义消息处理工具执行错误"""
    try:
        return handler(request)
    except Exception as e:
        # 返回自定义错误消息给模型
        return ToolMessage(
            content=f"Tool error: Please check your input and try again. ({str(e)})",
            tool_call_id=request.tool_call["id"]
        )

agent = create_agent(
    model="gpt-4.1",
    tools=[search, get_weather],
    middleware=[handle_tool_errors]
)
```

### 3.3 系统提示 (System Prompt)

#### 基础用法

```python
# 简单字符串
agent = create_agent(
    model,
    tools,
    system_prompt="You are a helpful assistant. Be concise and accurate."
)
```

#### 使用 SystemMessage

```python
from langchain.agents import create_agent
from langchain.messages import SystemMessage, HumanMessage

literary_agent = create_agent(
    model="anthropic:claude-sonnet-4-5",
    system_prompt=SystemMessage(
        content=[
            {
                "type": "text",
                "text": "You are an AI assistant tasked with analyzing literary works.",
            },
            {
                "type": "text",
                "text": "<the entire contents of 'Pride and Prejudice'>",
                "cache_control": {"type": "ephemeral"}  # Anthropic 提示缓存
            }
        ]
    )
)

result = literary_agent.invoke({
    "messages": [HumanMessage("Analyze the major themes in 'Pride and Prejudice'.")]
})
```

#### 动态系统提示

```python
from typing import TypedDict
from langchain.agents import create_agent
from langchain.agents.middleware import dynamic_prompt, ModelRequest

class Context(TypedDict):
    user_role: str

@dynamic_prompt
def user_role_prompt(request: ModelRequest) -> str:
    """根据用户角色生成系统提示"""
    user_role = request.runtime.context.get("user_role", "user")
    base_prompt = "You are a helpful assistant."

    if user_role == "expert":
        return f"{base_prompt} Provide detailed technical responses."
    elif user_role == "beginner":
        return f"{base_prompt} Explain concepts simply and avoid jargon."

    return base_prompt

agent = create_agent(
    model="gpt-4.1",
    tools=[web_search],
    middleware=[user_role_prompt],
    context_schema=Context
)

# 调用时传入上下文
result = agent.invoke(
    {"messages": [{"role": "user", "content": "Explain machine learning"}]},
    context={"user_role": "expert"}
)
```

### 3.4 命名

```python
agent = create_agent(
    model,
    tools,
    name="research_assistant"
)
```

> **最佳实践：** 使用 snake_case 命名 agent 和工具。避免空格和特殊字符，确保跨 provider 兼容性。

---

## 4. 调用 Agent

### 基本调用

```python
result = agent.invoke({
    "messages": [{"role": "user", "content": "What's the weather in San Francisco?"}]
})
```

### 流式输出

```python
from langchain.messages import AIMessage, HumanMessage

for chunk in agent.stream(
    {"messages": [{"role": "user", "content": "Search for AI news and summarize the findings"}]},
    stream_mode="values"
):
    # 每个 chunk 包含该时间点的完整状态
    latest_message = chunk["messages"][-1]
    if latest_message.content:
        if isinstance(latest_message, HumanMessage):
            print(f"User: {latest_message.content}")
        elif isinstance(latest_message, AIMessage):
            print(f"Agent: {latest_message.content}")
    elif latest_message.tool_calls:
        print(f"Calling tools: {[tc['name'] for tc in latest_message.tool_calls]}")
```

Agent 支持 LangGraph Graph API 的所有方法，包括 `stream`、`invoke` 等。

---

## 5. 高级概念

### 5.1 结构化输出

在某些情况下，可能需要 agent 以特定格式返回输出。LangChain 通过 `response_format` 参数提供结构化输出策略。

#### ToolStrategy

使用人工工具调用生成结构化输出，适用于任何支持工具调用的模型。

```python
from pydantic import BaseModel
from langchain.agents import create_agent
from langchain.agents.structured_output import ToolStrategy

class ContactInfo(BaseModel):
    name: str
    email: str
    phone: str

agent = create_agent(
    model="gpt-4.1-mini",
    tools=[search_tool],
    response_format=ToolStrategy(ContactInfo)
)

result = agent.invoke({
    "messages": [{
        "role": "user",
        "content": "Extract contact info from: John Doe, john@example.com, (555) 123-4567"
    }]
})

# 访问结构化响应
print(result["structured_response"])
# ContactInfo(name='John Doe', email='john@example.com', phone='(555) 123-4567')
```

#### ProviderStrategy

使用模型 provider 的原生结构化输出生成，更可靠但仅适用于支持原生结构化输出的 provider。

```python
from langchain.agents.structured_output import ProviderStrategy

agent = create_agent(
    model="gpt-4.1",
    response_format=ProviderStrategy(ContactInfo)
)
```

> **注意：** LangChain 1.0+ 中，直接传入 schema（如 `response_format=ContactInfo`）会默认使用 ProviderStrategy（如果模型支持），否则回退到 ToolStrategy。

### 5.2 记忆 (Memory)

Agent 通过消息状态自动维护对话历史。还可以配置自定义状态 schema 来记住额外信息。

自定义状态 schema 必须扩展 `AgentState`（作为 TypedDict）。

#### 通过 Middleware 定义状态（推荐）

```python
from langchain.agents import AgentState
from langchain.agents.middleware import AgentMiddleware
from typing import Any

class CustomState(AgentState):
    user_preferences: dict

class CustomMiddleware(AgentMiddleware):
    state_schema = CustomState
    tools = [tool1, tool2]

    def before_model(self, state: CustomState, runtime) -> dict[str, Any] | None:
        # 处理状态
        ...

agent = create_agent(
    model,
    tools=tools,
    middleware=[CustomMiddleware()]
)

# 调用时传入额外状态
result = agent.invoke({
    "messages": [{"role": "user", "content": "I prefer technical explanations"}],
    "user_preferences": {"style": "technical", "verbosity": "detailed"},
})
```

#### 通过 state_schema 定义状态

```python
from langchain.agents import AgentState

class CustomState(AgentState):
    user_preferences: dict

agent = create_agent(
    model,
    tools=[tool1, tool2],
    state_schema=CustomState
)
```

> **注意：** LangChain 1.0+ 中，自定义状态 schema 必须是 TypedDict 类型，不再支持 Pydantic 模型和 dataclass。

> **最佳实践：** 通过 middleware 定义自定义状态是首选方式，因为它允许将状态扩展概念上限定在相关的 middleware 和工具范围内。

### 5.3 长期记忆

对于跨会话持久化的长期记忆，参见 Long-term memory 文档。

---

## 6. Middleware

Middleware 提供强大的可扩展性，用于在 agent 执行的不同阶段自定义行为。

### 使用场景

- 在模型调用前处理状态（消息修剪、上下文注入）
- 修改或验证模型响应（guardrails、内容过滤）
- 自定义工具错误处理逻辑
- 实现动态模型选择
- 添加自定义日志、监控或分析

### 常用装饰器

| 装饰器 | 用途 |
|--------|------|
| `@before_model` | 在模型调用前处理状态 |
| `@after_model` | 在模型调用后修改响应 |
| `@wrap_model_call` | 包装模型调用，实现动态模型选择 |
| `@wrap_tool_call` | 包装工具调用，实现错误处理 |
| `@dynamic_prompt` | 动态生成系统提示 |

### 示例

```python
from langchain.agents.middleware import before_model, after_model

@before_model
def add_context(state, runtime):
    """在每次模型调用前添加上下文"""
    return {"messages": [{"role": "system", "content": "Remember to be helpful."}]}

@after_model
def validate_response(response, state, runtime):
    """验证模型响应"""
    if "I don't know" in response.content:
        # 触发重试或其他处理
        ...
```

---

## 7. 框架对比：LangChain vs LangGraph vs Deep Agents

| 框架 | 描述 | 适用场景 |
|------|------|----------|
| **Deep Agents** | 全功能 agent 框架，包含自动压缩长对话、虚拟文件系统、子 agent 生成 | 推荐首选，需要现代功能和开箱即用体验 |
| **LangChain** | 预构建 agent 架构和模型集成 | 快速构建 agents 和自主应用 |
| **LangGraph** | 低级 agent 编排框架和运行时 | 需要确定性和 agentic 工作流组合，需要大量自定义 |

**关系说明：**
- Deep Agents 是 LangChain agents 的实现
- LangChain agents 构建在 LangGraph 之上，提供持久执行、流式输出、人在环路、持久化等功能
- 基本使用 LangChain agent 不需要了解 LangGraph

---

## 8. 最佳实践

### 命名规范
- 使用 snake_case 命名 agent 和工具（如 `research_assistant`）
- 避免空格和特殊字符
- 仅使用字母数字、下划线和连字符

### 工具设计
- 为工具提供清晰的描述和参数说明
- 使用 `@tool` 装饰器自定义 schema
- 实现适当的错误处理

### 动态功能
- 根据认证状态、用户权限动态过滤工具
- 根据对话长度动态选择模型
- 根据上下文动态生成系统提示

### 调试和监控
```bash
# 设置 LangSmith 追踪
export LANGSMITH_TRACING=true
export LANGSMITH_API_KEY=your_api_key
```

### 性能优化
- 使用 Anthropic 提示缓存减少重复请求的延迟和成本
- 对于长对话，考虑使用 Deep Agents 的自动压缩功能
- 合理设置 `max_tokens` 和 `timeout`

---

## 参考资源

- [LangChain 官方文档](https://docs.langchain.com/oss/python/langchain/agents)
- [LangSmith](https://docs.langchain.com/langsmith/home) - 追踪、调试和评估 agents
- [LangGraph 文档](https://docs.langchain.com/oss/python/langgraph/overview)
- [Deep Agents 文档](https://docs.langchain.com/oss/python/deepagents/overview)
