# LangGraph 使用文档

> 基于 LangGraph 官方文档整理
> 最后更新：2026-03-20

---

## 目录

1. [概述](#1-概述)
2. [快速开始](#2-快速开始)
3. [Graph API](#3-graph-api)
4. [Functional API](#4-functional-api)
5. [持久化 (Persistence)](#5-持久化-persistence)
6. [中断与人机协同 (Interrupts)](#6-中断与人机协同-interrupts)
7. [记忆与存储 (Memory & Store)](#7-记忆与存储-memory--store)
8. [API 对比与选择](#8-api-对比与选择)

---

## 1. 概述

LangGraph 是一个用于构建、管理和部署长运行、有状态 agent 的低级编排框架和运行时。它由 LangChain Inc 构建，但可以独立于 LangChain 使用。

### 核心特点

- **持久执行 (Durable Execution)**: 构建能够持久化并通过故障恢复的 agent，可以长时间运行并从断点恢复
- **人机协同 (Human-in-the-loop)**: 在任意检查点检查、中断和修改 agent 状态，实现人工审核
- **全面记忆 (Comprehensive Memory)**: 创建具有短期工作记忆和跨会话长期记忆的有状态 agent
- **流式输出 (Streaming)**: 支持实时流式输出 agent 执行过程
- **时间旅行 (Time Travel)**: 回放先前的图执行，审查和调试特定步骤
- **子图 (Subgraphs)**: 支持嵌套子图，实现多 agent 系统

### LangGraph 生态系统

| 产品 | 描述 |
|------|------|
| **LangGraph** | 低级 agent 编排框架 |
| **LangChain** | 提供构建在 LangGraph 之上的 agent 抽象 |
| **Deep Agents** | 全功能 agent 框架，推荐首选 |
| **LangSmith** | 追踪、评估、监控和部署 |

---

## 2. 快速开始

### 安装

```bash
pip install -U langgraph
```

### Hello World 示例

```python
from langgraph.graph import StateGraph, MessagesState, START, END

def mock_llm(state: MessagesState):
    return {"messages": [{"role": "ai", "content": "hello world"}]}

# 构建图
graph = StateGraph(MessagesState)
graph.add_node(mock_llm)
graph.add_edge(START, "mock_llm")
graph.add_edge("mock_llm", END)

# 编译图
graph = graph.compile()

# 执行
result = graph.invoke({"messages": [{"role": "user", "content": "hi!"}]})
```

### 完整 Agent 示例（计算器）

```python
from langchain.tools import tool
from langchain.chat_models import init_chat_model
from langchain.messages import AnyMessage, SystemMessage, HumanMessage, ToolMessage
from langgraph.graph import StateGraph, START, END
from typing import Annotated, Literal
from typing_extensions import TypedDict
import operator

# 1. 定义工具和模型
@tool
def multiply(a: int, b: int) -> int:
    """Multiply a and b."""
    return a * b

@tool
def add(a: int, b: int) -> int:
    """Add a and b."""
    return a + b

@tool
def divide(a: int, b: int) -> float:
    """Divide a and b."""
    return a / b

model = init_chat_model("claude-sonnet-4-6", temperature=0)
tools = [add, multiply, divide]
tools_by_name = {tool.name: tool for tool in tools}
model_with_tools = model.bind_tools(tools)

# 2. 定义状态
class MessagesState(TypedDict):
    messages: Annotated[list[AnyMessage], operator.add]
    llm_calls: int

# 3. 定义模型节点
def llm_call(state: dict):
    return {
        "messages": [
            model_with_tools.invoke(
                [SystemMessage(content="You are a helpful assistant.")]
                + state["messages"]
            )
        ],
        "llm_calls": state.get("llm_calls", 0) + 1
    }

# 4. 定义工具节点
def tool_node(state: dict):
    result = []
    for tool_call in state["messages"][-1].tool_calls:
        tool = tools_by_name[tool_call["name"]]
        observation = tool.invoke(tool_call["args"])
        result.append(ToolMessage(
            content=observation,
            tool_call_id=tool_call["id"]
        ))
    return {"messages": result}

# 5. 定义路由逻辑
def should_continue(state: MessagesState) -> Literal["tool_node", END]:
    last_message = state["messages"][-1]
    if last_message.tool_calls:
        return "tool_node"
    return END

# 6. 构建并编译图
builder = StateGraph(MessagesState)
builder.add_node("llm_call", llm_call)
builder.add_node("tool_node", tool_node)
builder.add_edge(START, "llm_call")
builder.add_conditional_edges("llm_call", should_continue)
builder.add_edge("tool_node", "llm_call")

graph = builder.compile()

# 7. 执行
result = graph.invoke({
    "messages": [HumanMessage(content="Add 3 and 4.")]
})
```

---

## 3. Graph API

### 核心概念

LangGraph 将 agent 工作流程建模为图，由三个关键组件构成：

| 组件 | 描述 |
|------|------|
| **State** | 共享数据结构，表示应用的当前快照 |
| **Nodes** | 编码 agent 逻辑的函数，接收状态、执行计算、返回更新 |
| **Edges** | 根据当前状态决定下一个执行哪个节点的函数 |

### StateGraph

#### 定义状态

```python
from typing import Annotated
from typing_extensions import TypedDict
from operator import add

class State(TypedDict):
    foo: int
    bar: Annotated[list[str], add]  # 使用 reducer 累加列表
```

#### Reducers

Reducer 定义如何应用状态更新：

```python
# 默认 reducer（覆盖）
class State(TypedDict):
    foo: int  # 更新会覆盖原值

# 使用 Annotated 指定 reducer
class State(TypedDict):
    bar: Annotated[list[str], add]  # 更新会累加到列表
```

#### MessagesState（预定义状态）

```python
from langgraph.graph import MessagesState

class State(MessagesState):
    documents: list[str]  # 扩展额外字段
```

### Nodes

节点是接受 state 作为参数的函数：

```python
from langchain_core.runnables import RunnableConfig
from langgraph.runtime import Runtime

def plain_node(state: State):
    return {"result": "hello"}

def node_with_config(state: State, config: RunnableConfig):
    thread_id = config["configurable"]["thread_id"]
    return {"result": f"Hello from thread {thread_id}"}

def node_with_runtime(state: State, runtime: Runtime):
    # 访问 store 或其他运行时上下文
    return {"result": "hello"}
```

### Edges

#### 普通边

```python
graph.add_edge("node_a", "node_b")
```

#### 条件边

```python
def routing_function(state: State) -> str:
    if some_condition:
        return "node_b"
    return "node_c"

graph.add_conditional_edges("node_a", routing_function)

# 或映射到具体节点名
graph.add_conditional_edges("node_a", routing_function, {
    True: "node_b",
    False: "node_c"
})
```

#### 入口点

```python
from langgraph.graph import START, END

graph.add_edge(START, "node_a")  # 第一个节点
graph.add_edge("node_b", END)     # 终止节点
```

#### Send（动态边）

用于 map-reduce 模式：

```python
from langgraph.types import Send

def continue_to_jokes(state: State):
    return [Send("generate_joke", {"subject": s}) for s in state['subjects']]

graph.add_conditional_edges("node_a", continue_to_jokes)
```

### Command

Command 用于组合状态更新和控制流：

```python
from langgraph.types import Command, interrupt

def my_node(state: State) -> Command[Literal["next_node"]]:
    return Command(
        update={"foo": "bar"},  # 状态更新
        goto="next_node"         # 路由到下一个节点
    )

# 从中断恢复
def human_review(state: State):
    answer = interrupt("Do you approve?")
    return {"messages": [{"role": "user", "content": answer}]}

# 恢复执行
graph.invoke(Command(resume="yes"), config)
```

### 编译图

```python
from langgraph.checkpoint.memory import InMemorySaver

checkpointer = InMemorySaver()
graph = builder.compile(checkpointer=checkpointer)
```

### 执行图

```python
config = {"configurable": {"thread_id": "1"}}

# 同步执行
result = graph.invoke({"messages": [...]}, config)

# 流式输出
for chunk in graph.stream({"messages": [...]}, config):
    print(chunk)

# 异步执行
result = await graph.ainvoke({"messages": [...]}, config)
```

### 节点缓存

```python
from langgraph.cache.memory import InMemoryCache
from langgraph.types import CachePolicy

graph = builder.compile(
    cache=InMemoryCache(),
    cache_policy=CachePolicy(ttl=3)  # 3 秒缓存
)
```

---

## 4. Functional API

Functional API 允许使用标准 Python 函数定义工作流，添加持久化、记忆、人机协同和流式输出功能。

### 核心构建块

| 装饰器 | 描述 |
|--------|------|
| `@entrypoint` | 标记函数为工作流入口点，管理执行流 |
| `@task` | 表示离散工作单元，可异步执行 |

### 定义 Entrypoint

```python
from langgraph.func import entrypoint
from langgraph.checkpoint.memory import InMemorySaver

checkpointer = InMemorySaver()

@entrypoint(checkpointer=checkpointer)
def my_workflow(some_input: dict) -> int:
    # 工作流逻辑
    ...
    return result
```

### 定义 Task

```python
from langgraph.func import task

@task()
def slow_computation(input_value):
    # 长时间运行的操作
    ...
    return result
```

### 执行 Task

```python
@entrypoint(checkpointer=checkpointer)
def my_workflow(some_input: int) -> int:
    future = slow_computation(some_input)
    return future.result()  # 同步等待结果
```

### 执行 Entrypoint

```python
config = {"configurable": {"thread_id": "some_thread_id"}}

# 同步执行
result = my_workflow.invoke(some_input, config)

# 流式输出
for chunk in my_workflow.stream(some_input, config):
    print(chunk)

# 异步执行
result = await my_workflow.ainvoke(some_input, config)
```

### 中断与人机协同

```python
from langgraph.func import entrypoint, task
from langgraph.types import interrupt

@task
def write_essay(topic: str) -> str:
    return f"An essay about topic: {topic}"

@entrypoint(checkpointer=InMemorySaver())
def workflow(topic: str) -> dict:
    essay = write_essay(topic).result()

    # 中断并等待人工审核
    is_approved = interrupt({
        "essay": essay,
        "action": "Please approve/reject the essay"
    })

    return {
        "essay": essay,
        "is_approved": is_approved
    }

# 第一次执行（会中断）
config = {"configurable": {"thread_id": "1"}}
result = workflow.invoke("cat", config)

# 恢复执行
from langgraph.types import Command
result = workflow.invoke(Command(resume=True), config)
```

### 短期记忆

```python
@entrypoint(checkpointer=checkpointer)
def my_workflow(number: int, *, previous: Any = None) -> int:
    previous = previous or 0
    return number + previous

config = {"configurable": {"thread_id": "1"}}
my_workflow.invoke(1, config)  # 返回 1
my_workflow.invoke(2, config)  # 返回 3 (previous=1)
```

### entrypoint.final

```python
from langgraph.func import entrypoint

@entrypoint(checkpointer=checkpointer)
def my_workflow(number: int, *, previous: Any = None) -> entrypoint.final[int, int]:
    previous = previous or 0
    # 返回 previous 给调用者，保存 2*number 到检查点
    return entrypoint.final(value=previous, save=2 * number)
```

---

## 5. 持久化 (Persistence)

### 为什么使用持久化

持久化保存图状态为检查点，支持以下功能：

- **人机协同**: 检查、中断和批准图步骤
- **记忆**: 跨交互保留对话历史
- **时间旅行**: 回放先前的执行进行调试
- **容错**: 从最后一个成功步骤恢复执行

### 核心概念

#### Threads

```python
config = {"configurable": {"thread_id": "1"}}
```

#### Checkpoints

每个 super-step 边界保存一个检查点（状态快照）。

#### 获取状态

```python
# 获取最新状态
config = {"configurable": {"thread_id": "1"}}
snapshot = graph.get_state(config)

# 获取特定检查点
config = {"configurable": {"thread_id": "1", "checkpoint_id": "..."}}
snapshot = graph.get_state(config)
```

#### StateSnapshot 字段

| 字段 | 类型 | 描述 |
|------|------|------|
| `values` | dict | 状态通道的值 |
| `next` | tuple[str, ...] | 下一个要执行的节点 |
| `config` | dict | 包含 thread_id、checkpoint_ns、checkpoint_id |
| `metadata` | dict | 执行元数据（source、writes、step） |
| `created_at` | str | ISO 8601 时间戳 |
| `parent_config` | dict | 上一个检查点的配置 |
| `tasks` | tuple | 要执行的任务 |

#### 获取状态历史

```python
config = {"configurable": {"thread_id": "1"}}
history = list(graph.get_state_history(config))

# 查找特定检查点
before_node_b = next(s for s in history if s.next == ("node_b",))
step_2 = next(s for s in history if s.metadata["step"] == 2)
```

### 更新状态

```python
graph.update_state(config, {"foo": "new_value"})
```

### Checkpointer 实现

| Checkpointer | 描述 |
|--------------|------|
| `InMemorySaver` | 内存检查点器，适合实验 |
| `SqliteSaver` | SQLite 数据库，适合本地开发 |
| `PostgresSaver` | PostgreSQL 数据库，适合生产 |
| `CosmosDBSaver` | Azure Cosmos DB，适合 Azure 生产环境 |

### 加密

```python
from langgraph.checkpoint.serde.encrypted import EncryptedSerializer

serde = EncryptedSerializer.from_pycryptodome_aes()
checkpointer = SqliteSaver(sqlite3.connect("checkpoint.db"), serde=serde)
```

---

## 6. 中断与人机协同 (Interrupts)

### 使用 interrupt

```python
from langgraph.types import interrupt

def human_review(state: State):
    # 暂停并等待值
    answer = interrupt("Do you approve?")
    return {"messages": [{"role": "user", "content": answer}]}
```

### 中断后恢复

```python
# 第一次执行（中断）
result = graph.invoke({"messages": [...]}, config)

# 恢复执行
result = graph.invoke(Command(resume="yes"), config)
```

### 多个中断

```python
def multi_step_review(state: State):
    step1 = interrupt("Review step 1")
    step2 = interrupt("Review step 2")
    return {"step1": step1, "step2": step2}
```

### 在 Functional API 中使用中断

```python
from langgraph.func import entrypoint, task
from langgraph.types import interrupt

@entrypoint(checkpointer=InMemorySaver())
def workflow(input_data: str):
    result = task_function(input_data).result()
    approval = interrupt({"result": result, "action": "approve?"})
    return {"result": result, "approved": approval}
```

---

## 7. 记忆与存储 (Memory & Store)

### Store 基础用法

```python
from langgraph.store.memory import InMemoryStore

store = InMemoryStore()

# 命名空间
user_id = "1"
namespace = (user_id, "memories")

# 存储记忆
import uuid
memory_id = str(uuid.uuid4())
memory = {"food_preference": "I like pizza"}
store.put(namespace, memory_id, memory)

# 搜索记忆
memories = store.search(namespace)
latest = memories[-1].value
```

### 语义搜索

```python
from langchain.embeddings import init_embeddings

store = InMemoryStore(
    index={
        "embed": init_embeddings("openai:text-embedding-3-small"),
        "dims": 1536,
        "fields": ["food_preference", "$"]
    }
)

# 语义搜索
memories = store.search(
    namespace,
    query="What does the user like to eat?",
    limit=3
)
```

### 在 LangGraph 中使用 Store

```python
from dataclasses import dataclass
from langgraph.checkpoint.memory import InMemorySaver
from langgraph.runtime import Runtime

@dataclass
class Context:
    user_id: str

checkpointer = InMemorySaver()
store = InMemoryStore()

@entrypoint(checkpointer=checkpointer, store=store)
def workflow(user_input: str, *, context: Context):
    # 访问 store
    namespace = (context.user_id, "memories")
    memories = store.search(namespace)
    # ... 使用记忆
    return {"result": "done"}
```

### 在节点中访问 Store

```python
from langgraph.runtime import Runtime

async def update_memory(state: MessagesState, runtime: Runtime[Context]):
    user_id = runtime.context.user_id
    namespace = (user_id, "memories")

    # 分析对话并创建新记忆
    memory = {"topic": "food", "preference": "Italian"}
    memory_id = str(uuid.uuid4())

    await runtime.store.aput(namespace, memory_id, memory)

async def call_model(state: MessagesState, runtime: Runtime[Context]):
    user_id = runtime.context.user_id
    namespace = (user_id, "memories")

    # 语义搜索相关记忆
    memories = await runtime.store.asearch(
        namespace,
        query=state["messages"][-1].content,
        limit=3
    )

    # 在模型调用中使用记忆
    ...
```

---

## 8. API 对比与选择

### Graph API vs Functional API

| 特性 | Graph API | Functional API |
|------|-----------|----------------|
| **控制流** | 声明式图结构 | 标准 Python 控制流 |
| **状态管理** | 显式 State 和 reducers | 函数作用域状态 |
| **检查点** | 每个 super-step 后创建 | 任务执行时创建 |
| **可视化** | 支持图可视化 | 不支持（动态生成） |
| **适用场景** | 需要清晰图结构 | 偏好函数式风格 |

### 选择建议

| 需求 | 推荐方案 |
|------|----------|
| 快速构建 agent | **LangChain** `create_agent` |
| 需要现代功能（自动压缩、虚拟文件系统等） | **Deep Agents** |
| 需要完全控制和自定义 | **LangGraph** |
| 偏好声明式图结构 | **LangGraph Graph API** |
| 偏好函数式编程 | **LangGraph Functional API** |

---

## 参考资源

- [LangGraph 官方文档](https://docs.langchain.com/oss/python/langgraph/overview)
- [Graph API 参考](https://docs.langchain.com/oss/python/langgraph/graph-api)
- [Functional API 参考](https://docs.langchain.com/oss/python/langgraph/functional-api)
- [LangSmith 文档](https://docs.langchain.com/langsmith/home)
