# LangChain 中使用 Qwen LLM Provider

本文档介绍如何在 LangChain 中使用阿里巴巴通义千问（Qwen）大语言模型。

## 概述

LangChain 提供了两种主要方式来集成 Qwen 模型：

1. **ChatQwen** - 官方推荐方式，使用 `langchain-qwq` 包
2. **ChatTongyi** - 社区集成方式，使用 `langchain-community` 包

## 方式一：ChatQwen（推荐）

### 简介

`ChatQwen` 是 LangChain 官方提供的 Qwen 集成，位于独立的 `langchain-qwq` 包中。

### 模型特性

| 功能 | 支持情况 |
|------|----------|
| Tool Calling | ✅ |
| Structured Output | ✅ |
| Image Input | ✅ |
| Video Input | ✅ |
| Token-level Streaming | ✅ |
| Token Usage | ✅ |
| Logprobs | ❌ |
| Native Async | ❌ |

### 安装

```bash
pip install -qU langchain-qwq
```

### 配置 API Key

访问 [Alibaba API Key 页面](https://account.alibabacloud.com/login/login.htm?oauth_callback=https%3A%2F%2Fbailian.console.alibabacloud.com%2F%3FapiKey%3D1&lang=en#/api-key) 创建阿里云账户并生成 API Key。

```python
import getpass
import os

if not os.getenv("DASHSCOPE_API_KEY"):
    os.environ["DASHSCOPE_API_KEY"] = getpass.getpass("Enter your Dashscope API key: ")
```

### 基本使用

```python
from langchain_qwq import ChatQwen

llm = ChatQwen(
    model="qwen-flash",
    max_tokens=3000,
    timeout=None,
    max_retries=2,
)

messages = [
    (
        "system",
        "You are a helpful assistant that translates English to French. Translate the user sentence.",
    ),
    ("human", "I love programming."),
]

ai_msg = llm.invoke(messages)
print(ai_msg)
```

输出：
```
AIMessage(content="J'adore la programmation.", additional_kwargs={}, response_metadata={'finish_reason': 'stop', 'model_name': 'qwen-flash'}, id='run--...', usage_metadata={'input_tokens': 32, 'output_tokens': 8, 'total_tokens': 40})
```

### Tool Calling

```python
from langchain.tools import tool
from langchain_qwq import ChatQwen

@tool
def multiply(first_int: int, second_int: int) -> int:
    """Multiply two integers together."""
    return first_int * second_int

llm = ChatQwen(model="qwen-flash")
llm_with_tools = llm.bind_tools([multiply])

msg = llm_with_tools.invoke("What's 5 times forty two")
print(msg)
```

输出：
```
content='' additional_kwargs={'tool_calls': [{'index': 0, 'id': 'call_...', 'function': {'arguments': '{"first_int": 5, "second_int": 42}', 'name': 'multiply'}, 'type': 'function'}]} tool_calls=[{'name': 'multiply', 'args': {'first_int': 5, 'second_int': 42}, 'id': 'call_...', 'type': 'tool_call'}]
```

### 视觉支持（多模态）

#### 图片输入

```python
from langchain.messages import HumanMessage
from langchain_qwq import ChatQwen

model = ChatQwen(model="qwen-vl-max-latest")

messages = [
    HumanMessage(
        content=[
            {
                "type": "image_url",
                "image_url": {
                    "url": "https://example.com/image/image.png"
                },
            },
            {"type": "text", "text": "What do you see in this image?"},
        ])
]

response = model.invoke(messages)
print(response)
```

#### 视频输入

```python
from langchain.messages import HumanMessage
from langchain_qwq import ChatQwen

model = ChatQwen(model="qwen-vl-max-latest")

messages = [
    HumanMessage(
        content=[
            {
                "type": "video_url",
                "video_url": {
                    "url": "https://example.com/video/1.mp4"
                },
            },
            {"type": "text", "text": "Can you tell me about this video?"},
        ])
]

response = model.invoke(messages)
print(response)
```

### 可用模型

- `qwen-flash` - 快速响应模型
- `qwen-vl-max-latest` - 视觉语言模型（支持图片和视频）

---

## 方式二：ChatTongyi（社区集成）

### 简介

`ChatTongyi` 是通义千问的社区集成版本，位于 `langchain-community` 包中。通义千问是由阿里巴巴达摩院开发的大语言模型。

### 安装

```bash
pip install -qU dashscope
```

### 获取 API Key

访问：https://help.aliyun.com/document_detail/611472.html

```python
from getpass import getpass
import os

DASHSCOPE_API_KEY = getpass()
os.environ["DASHSCOPE_API_KEY"] = DASHSCOPE_API_KEY
```

### 基本使用

```python
from langchain_community.chat_models.tongyi import ChatTongyi
from langchain.messages import HumanMessage

chatLLM = ChatTongyi(streaming=True)
res = chatLLM.stream([HumanMessage(content="hi")], streaming=True)
for r in res:
    print("chat resp:", r)
```

### 流式响应

```python
from langchain.messages import HumanMessage, SystemMessage

messages = [
    SystemMessage(
        content="You are a helpful assistant that translates English to French."
    ),
    HumanMessage(
        content="Translate this sentence from English to French. I love programming."
    ),
]

chatLLM = ChatTongyi()
response = chatLLM.invoke(messages)
print(response)
```

输出：
```
AIMessage(content="J'adore programmer.", response_metadata={'model_name': 'qwen-turbo', 'finish_reason': 'stop', ...})
```

### Tool Calling

```python
from langchain_community.chat_models.tongyi import ChatTongyi
from langchain.tools import tool

@tool
def multiply(first_int: int, second_int: int) -> int:
    """Multiply two integers together."""
    return first_int * second_int

llm = ChatTongyi(model="qwen-turbo")
llm_with_tools = llm.bind_tools([multiply])

msg = llm_with_tools.invoke("What's 5 times forty two")
print(msg)
```

### 手动构造 Tool 参数

```python
from langchain_community.chat_models.tongyi import ChatTongyi
from langchain.messages import HumanMessage, SystemMessage

tools = [
    {
        "type": "function",
        "function": {
            "name": "get_current_time",
            "description": "当你想知道现在的时间时非常有用。",
            "parameters": {},
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_current_weather",
            "description": "当你想查询指定城市的天气时非常有用。",
            "parameters": {
                "type": "object",
                "properties": {
                    "location": {
                        "type": "string",
                        "description": "城市或县区，比如北京市、杭州市、余杭区等。",
                    }
                },
            },
            "required": ["location"],
        },
    },
]

messages = [
    SystemMessage(content="You are a helpful assistant."),
    HumanMessage(content="What is the weather like in San Francisco?"),
]

chatLLM = ChatTongyi()
llm_kwargs = {"tools": tools, "result_format": "message"}
ai_message = chatLLM.bind(**llm_kwargs).invoke(messages)
print(ai_message)
```

### 续写模式（Partial Mode）

```python
from langchain_community.chat_models.tongyi import ChatTongyi
from langchain.messages import HumanMessage, AIMessage

messages = [
    HumanMessage(
        content="Please continue the sentence 'Spring has arrived, and the earth' to express the beauty of spring."
    ),
    AIMessage(
        content="Spring has arrived, and the earth",
        additional_kwargs={"partial": True}
    ),
]

chatLLM = ChatTongyi()
ai_message = chatLLM.invoke(messages)
print(ai_message)
```

### 视觉支持

```python
from langchain_community.chat_models import ChatTongyi
from langchain.messages import HumanMessage

chatLLM = ChatTongyi(model_name="qwen-vl-max")

image_message = {
    "image": "https://lilianweng.github.io/posts/2023-06-23-agent/agent-overview.png",
}
text_message = {
    "text": "summarize this picture",
}

message = HumanMessage(content=[text_message, image_message])
response = chatLLM.invoke([message])
print(response)
```

### 可用模型

- `qwen-turbo` - 基础模型
- `qwen-plus` - 增强模型
- `qwen-max` - 最强模型
- `qwen-vl-plus` - 视觉语言模型
- `qwen-vl-max` - 高级视觉语言模型

---

## 两种方式的对比

| 特性 | ChatQwen (langchain-qwq) | ChatTongyi (langchain-community) |
|------|--------------------------|----------------------------------|
| 包类型 | 官方独立包 | 社区集成包 |
| 安装命令 | `pip install langchain-qwq` | `pip install dashscope` |
| 导入路径 | `from langchain_qwq import ChatQwen` | `from langchain_community.chat_models.tongyi import ChatTongyi` |
| 流式响应 | ✅ | ✅ |
| Tool Calling | ✅ | ✅ |
| 视觉支持 | ✅ | ✅ |
| 视频支持 | ✅ | ❌ |
| 推荐程度 | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ |

## 推荐

**推荐使用 ChatQwen (langchain-qwq)**，原因：
- 官方维护，更新更及时
- 独立的包，依赖更清晰
- 支持视频输入等多模态功能
- 更好的 LangChain 生态整合

## 参考链接

- [ChatQwen 官方文档](https://docs.langchain.com/oss/python/integrations/chat/qwen)
- [ChatTongyi 官方文档](https://docs.langchain.com/oss/python/integrations/chat/tongyi)
- [阿里云 DashScope 文档](https://help.aliyun.com/document_detail/611472.html)
- [langchain-qwq PyPI](https://pypi.org/project/langchain-qwq/)
