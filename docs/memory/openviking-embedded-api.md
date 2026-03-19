# OpenViking Embedded API 文档总结

## 概述

OpenViking 是为 AI Agent 设计的上下文数据库，将所有上下文（Memory、Resource、Skill）统一抽象为目录结构，支持语义检索和渐进式内容加载。

## 连接模式

| 模式 | 使用场景 | 说明 |
|------|----------|------|
| **嵌入式** | 本地开发，单进程 | 本地运行，数据存储在本地 |
| **HTTP** | 连接 OpenViking Server | 通过 HTTP API 连接远程服务 |
| **CLI** | Shell 脚本、Agent 工具调用 | 通过 CLI 命令连接服务端 |

## 嵌入式模式初始化

```python
import openviking as ov

# 初始化客户端
client = ov.OpenViking(path="./data")
client.initialize()

# 使用客户端...

# 关闭连接
client.close()
```

配置文件路径默认为 `~/.openviking/ov.conf`，可通过环境变量指定：

```bash
export OPENVIKING_CONFIG_FILE=/path/to/ov.conf
```

最小配置示例：

```json
{
  "embedding": {
    "dense": {
      "api_base": "<api-endpoint>",
      "api_key": "<your-api-key>",
      "provider": "<volcengine|openai|jina>",
      "dimension": 1024,
      "model": "<model-name>"
    }
  },
  "vlm": {
    "api_base": "<api-endpoint>",
    "api_key": "<your-api-key>",
    "provider": "<volcengine|openai|jina>",
    "model": "<model-name>"
  }
}
```

---

## 核心概念

### Viking URI

所有内容的统一资源标识符，格式：`viking://{scope}/{path}`

**作用域（Scope）：**

| 作用域 | 说明 | 示例 |
|--------|------|------|
| `resources` | 独立资源 | `viking://resources/my-project/` |
| `user` | 用户级数据 | `viking://user/memories/` |
| `agent` | Agent 级数据 | `viking://agent/skills/` |
| `session` | 会话级数据 | `viking://session/{id}/` |

### 上下文类型（Context Types）

| 类型 | 用途 | 生命周期 | 存储位置 |
|------|------|----------|----------|
| **Resource** | 知识和规则 | 长期，相对静态 | `viking://resources/` |
| **Memory** | Agent 的认知 | 长期，动态更新 | `viking://user/memories/` / `viking://agent/memories/` |
| **Skill** | 可调用的能力 | 长期，静态 | `viking://agent/skills/` |

### 上下文层级（L0/L1/L2）

| 层级 | 名称 | 文件 | Token 限制 | 用途 |
|------|------|------|-----------|------|
| **L0** | 摘要 | `.abstract.md` | ~100 tokens | 向量搜索、快速过滤 |
| **L1** | 概览 | `.overview.md` | ~2k tokens | Rerank 精排、内容导航 |
| **L2** | 详情 | 原始文件/子目录 | 无限制 | 完整内容、按需加载 |

---

## API 参考

### 文件系统操作

#### abstract() - 读取 L0 摘要

```python
abstract = client.abstract("viking://resources/docs/")
```

#### overview() - 读取 L1 概览

```python
overview = client.overview("viking://resources/docs/")
```

#### read() - 读取 L2 完整内容

```python
content = client.read("viking://resources/docs/api.md")
```

#### ls() - 列出目录内容

```python
# 基本列表
entries = client.ls("viking://resources/")

# 简单路径列表
paths = client.ls("viking://resources/", simple=True)

# 递归列表
all_entries = client.ls("viking://resources/", recursive=True)
```

**条目结构：**

```python
{
    "name": "docs",
    "size": 4096,
    "isDir": True,
    "uri": "viking://resources/docs/",
    "modTime": "2024-01-01T00:00:00Z"
}
```

#### tree() - 获取目录树结构

```python
entries = client.tree("viking://resources/")
for entry in entries:
    print(f"{entry['rel_path']} - {'dir' if entry['isDir'] else 'file'}")
```

#### stat() - 获取状态信息

```python
info = client.stat("viking://resources/docs/api.md")
print(f"Size: {info['size']}")
```

#### mkdir() - 创建目录

```python
client.mkdir("viking://resources/new-project/")
```

#### rm() - 删除文件或目录

```python
# 删除单个文件
client.rm("viking://resources/docs/old.md")

# 递归删除目录
client.rm("viking://resources/old-project/", recursive=True)
```

#### mv() - 移动文件或目录

```python
client.mv(
    "viking://resources/old-name/",
    "viking://resources/new-name/"
)
```

---

### 资源管理

#### add_resource() - 添加资源

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| path | str | 是 | 本地文件路径、目录路径或 URL |
| target | str | 否 | 目标 Viking URI |
| reason | str | 否 | 添加该资源的原因 |
| wait | bool | 否 | 等待语义处理完成 |
| timeout | float | 否 | 超时时间（秒） |
| watch_interval | float | 否 | 定时更新间隔（分钟） |

```python
# 添加本地文件
result = client.add_resource("./documents/guide.md")
print(f"Added: {result['root_uri']}")

# 添加 URL
result = client.add_resource(
    "https://example.com/api-docs.md",
    target="viking://resources/external/",
    reason="External API documentation"
)

# 等待处理完成
result = client.add_resource("./documents/guide.md", wait=True)

# 批量添加后单独等待
client.add_resource("./file1.md")
client.add_resource("./file2.md")
status = client.wait_processed()
```

**支持的格式：**

| 格式 | 扩展名 | 处理方式 |
|------|--------|----------|
| PDF | `.pdf` | 文本和图像提取 |
| Markdown | `.md` | 原生支持 |
| HTML | `.html`, `.htm` | 清洗后文本提取 |
| 纯文本 | `.txt` | 直接导入 |
| JSON/YAML | `.json`, `.yaml`, `.yml` | 结构化解析 |
| 代码 | `.py`, `.js`, `.ts`, `.go` 等 | 语法感知解析 |
| 图像 | `.png`, `.jpg`, `.jpeg` 等 | VLM 描述 |
| 视频 | `.mp4`, `.mov`, `.avi` | 帧提取 + VLM |
| 音频 | `.mp3`, `.wav`, `.m4a` | 语音转录 |

---

### 技能管理

#### add_skill() - 添加技能

```python
# 字典格式
skill = {
    "name": "search-web",
    "description": "Search the web for current information",
    "content": "# search-web\n\n...",
    "allowed_tools": ["Tool1", "Tool2"],
    "tags": ["tag1", "tag2"]
}
result = client.add_skill(skill)

# MCP Tool 格式（自动转换）
mcp_tool = {
    "name": "calculator",
    "description": "Perform mathematical calculations",
    "inputSchema": {
        "type": "object",
        "properties": {
            "expression": {"type": "string", "description": "Math expression"}
        },
        "required": ["expression"]
    }
}
result = client.add_skill(mcp_tool)

# 从文件/目录添加
result = client.add_skill("./skills/search-web/SKILL.md")
result = client.add_skill("./skills/code-runner/")
```

#### 技能存储结构

```
viking://agent/skills/
+-- search-web/
|   +-- .abstract.md      # L0：简要描述
|   +-- .overview.md      # L1：参数和使用概览
|   +-- SKILL.md          # L2：完整文档
|   +-- [auxiliary files]  # 其他辅助文件
```

---

### 会话管理

#### 创建和使用会话

```python
from openviking.message import TextPart, ContextPart, ToolPart

# 创建新会话
session = client.session()
print(f"Session URI: {session.uri}")

# 加载已有会话
session = client.session(session_id="a1b2c3d4")
session.load()
```

#### add_message() - 添加消息

```python
# 添加用户消息
session.add_message("user", [
    TextPart(text="How do I authenticate users?")
])

# 添加带上下文引用的助手回复
session.add_message("assistant", [
    TextPart(text="Based on the documentation..."),
    ContextPart(
        uri="viking://resources/docs/auth/",
        context_type="resource",
        abstract="Authentication guide..."
    )
])

# 添加工具调用
session.add_message("assistant", [
    TextPart(text="Let me search for that..."),
    ToolPart(
        tool_id="call_123",
        tool_name="search_web",
        skill_uri="viking://skills/search-web/",
        tool_input={"query": "OAuth"},
        tool_output="Results...",
        tool_status="completed"
    )
])
```

#### used() - 记录使用情况

```python
# 记录使用的上下文
session.used(contexts=["viking://resources/docs/auth/"])

# 记录使用的技能
session.used(skill={
    "uri": "viking://skills/search-web/",
    "input": {"query": "OAuth"},
    "output": "Results...",
    "success": True
})
```

#### commit() - 提交会话

```python
result = session.commit()
print(f"Status: {result['status']}")
print(f"Memories extracted: {result['memories_extracted']}")
```

#### 会话属性

| 属性 | 类型 | 说明 |
|------|------|------|
| uri | str | 会话 Viking URI |
| messages | List[Message] | 当前消息 |
| stats | SessionStats | 会话统计信息 |
| summary | str | 压缩摘要 |
| usage_records | List[Usage] | 使用记录 |

#### 记忆分类

| 分类 | 位置 | 说明 |
|------|------|------|
| profile | `user/memories/.overview.md` | 用户个人信息 |
| preferences | `user/memories/preferences/` | 用户偏好 |
| entities | `user/memories/entities/` | 重要实体 |
| events | `user/memories/events/` | 重要事件 |
| cases | `agent/memories/cases/` | 问题-解决方案案例 |
| patterns | `agent/memories/patterns/` | 交互模式 |

---

### 检索

#### find() - 基本语义搜索

```python
results = client.find(
    "how to authenticate users",
    target_uri="viking://resources/",  # 可选，限制搜索范围
    limit=10,                           # 默认 10
    score_threshold=0.5                 # 可选
)

for ctx in results.resources:
    print(f"URI: {ctx.uri}")
    print(f"Score: {ctx.score:.3f}")
    print(f"Abstract: {ctx.abstract[:100]}...")
```

#### search() - 会话上下文搜索

```python
session = client.session()
session.add_message("user", [TextPart(text="I'm building a login page")])

results = client.search(
    "best practices",
    session=session
)
```

#### grep() - 模式搜索

```python
results = client.grep(
    "viking://resources/",
    "authentication",
    case_insensitive=True
)
print(f"Found {results['count']} matches")
```

#### glob() - 文件模式匹配

```python
results = client.glob("**/*.md", "viking://resources/")
print(f"Found {results['count']} markdown files")
```

#### FindResult 结构

```python
class FindResult:
    memories: List[MatchedContext]   # 记忆上下文
    resources: List[MatchedContext]  # 资源上下文
    skills: List[MatchedContext]     # 技能上下文
    total: int                       # 总数

class MatchedContext:
    uri: str                         # Viking URI
    context_type: str                # "resource"/"memory"/"skill"
    is_leaf: bool                    # 是否为叶子节点
    abstract: str                    # L0 内容
    score: float                     # 相关性分数 (0-1)
    match_reason: str                # 匹配原因
```

---

### 关联管理

#### link() - 创建关联

```python
# 单个关联
client.link(
    "viking://resources/docs/auth/",
    "viking://resources/docs/security/",
    reason="Security best practices"
)

# 多个关联
client.link(
    "viking://resources/docs/api/",
    ["viking://resources/docs/auth/", "viking://resources/docs/errors/"],
    reason="Related documentation"
)
```

#### relations() - 获取关联

```python
relations = client.relations("viking://resources/docs/auth/")
for rel in relations:
    print(f"{rel['uri']}: {rel['reason']}")
```

#### unlink() - 删除关联

```python
client.unlink(
    "viking://resources/docs/auth/",
    "viking://resources/docs/security/"
)
```

---

### 导入导出

#### export_ovpack() - 导出为 .ovpack

```python
path = client.export_ovpack(
    "viking://resources/my-project/",
    "./exports/my-project.ovpack"
)
```

#### import_ovpack() - 导入 .ovpack

```python
uri = client.import_ovpack(
    "./exports/my-project.ovpack",
    "viking://resources/imported/",
    force=True,
    vectorize=True
)
client.wait_processed()
```

---

### 系统监控

#### health() - 健康检查

```python
if client.observer.is_healthy():
    print("System OK")
```

#### status() - 系统状态

```python
print(client.observer.system)
```

#### wait_processed() - 等待处理完成

```python
status = client.wait_processed(timeout=60.0)
# Returns: {"pending": 0, "in_progress": 0, "processed": 20, "errors": 0}
```

#### Observer API

```python
# 队列状态
print(client.observer.queue)

# VikingDB 状态
print(client.observer.vikingdb)

# VLM 使用状态
print(client.observer.vlm)

# 整体系统状态
print(client.observer.system)
```

---

## 错误码

| 错误码 | HTTP 状态码 | 说明 |
|--------|-------------|------|
| `OK` | 200 | 成功 |
| `INVALID_ARGUMENT` | 400 | 无效参数 |
| `INVALID_URI` | 400 | 无效的 Viking URI 格式 |
| `NOT_FOUND` | 404 | 资源未找到 |
| `ALREADY_EXISTS` | 409 | 资源已存在 |
| `UNAUTHENTICATED` | 401 | 缺少或无效的 API Key |
| `PERMISSION_DENIED` | 403 | 权限不足 |
| `RESOURCE_EXHAUSTED` | 429 | 超出速率限制 |
| `FAILED_PRECONDITION` | 412 | 前置条件不满足 |
| `DEADLINE_EXCEEDED` | 504 | 操作超时 |
| `UNAVAILABLE` | 503 | 服务不可用 |
| `INTERNAL` | 500 | 内部服务器错误 |
| `EMBEDDING_FAILED` | 500 | Embedding 生成失败 |
| `VLM_FAILED` | 500 | VLM 调用失败 |
| `SESSION_EXPIRED` | 410 | 会话已过期 |

---

## 最佳实践

### 渐进式内容加载

```python
# 先用 L1 判断，仅在需要时加载 L2
results = client.find("authentication")

for ctx in results.resources:
    # L0 已包含在 ctx.abstract 中
    print(f"Abstract: {ctx.abstract}")

    if not ctx.is_leaf:
        # 获取 L1（概览）
        overview = client.overview(ctx.uri)
        if needs_more_detail(overview):
            # 加载 L2（完整内容）
            content = client.read(ctx.uri)
    else:
        # 叶子节点直接读取 L2
        content = client.read(ctx.uri)
```

### 按项目组织资源

```
viking://resources/
+-- project-a/
|   +-- docs/
|   +-- specs/
|   +-- references/
+-- project-b/
+-- shared/
    +-- common-docs/
```

### 会话管理

```python
# 在重要交互后提交
if len(session.messages) > 10:
    session.commit()

# 跟踪实际使用的内容
if context_was_useful:
    session.used(contexts=[ctx.uri])

# 恢复已有会话时先加载
session = client.session(session_id="existing-id")
session.load()
```

### 搜索优化

```python
# 使用具体的查询
results = client.find("OAuth 2.0 authorization code flow implementation")

# 限定搜索范围
results = client.find(
    "error handling",
    target_uri="viking://resources/my-project/"
)

# 对话式搜索使用会话上下文
results = client.search("best practices", session=session)
```

---

## 参考文档

- [OpenViking 中文文档](/home/rene/projs/OpenViking/docs/zh/)
- [API 概览](/home/rene/projs/OpenViking/docs/zh/api/01-overview.md)
- [资源管理](/home/rene/projs/OpenViking/docs/zh/api/02-resources.md)
- [文件系统](/home/rene/projs/OpenViking/docs/zh/api/03-filesystem.md)
- [技能管理](/home/rene/projs/OpenViking/docs/zh/api/04-skills.md)
- [会话管理](/home/rene/projs/OpenViking/docs/zh/api/05-sessions.md)
- [检索](/home/rene/projs/OpenViking/docs/zh/api/06-retrieval.md)
- [系统监控](/home/rene/projs/OpenViking/docs/zh/api/07-system.md)
- [管理员 API](/home/rene/projs/OpenViking/docs/zh/api/08-admin.md)