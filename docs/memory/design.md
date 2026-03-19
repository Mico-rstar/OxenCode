# OxenCode记忆系统设计
## Idea 
OxenCode的记忆系统 = 文件系统 + RAG + Agent记忆管理线程
OxenCode使用文件系统来阻止自身的记忆，记忆系统运行时动态检测记忆文件变更，自动触发向量索引构建

### 文件系统组织
```
memory
|- experience 遇到X case，应该怎么做
|- knowledge  陈述性事实性描述
|- notes      做了什么的凝练的日志摘要
|- histories  原始message轨迹
|- inner
    |- self   自我认知
    |- user   对用户的认知，包括用户偏好
```

### RAG系统
- RAG系统的管理范围包括：experience, knowledge, notes
- 技术栈：Chroma向量数据库，embbedding模型（调用云接口），langchain

### 记忆管理系统对外暴露接口
1. trigger_memory: 快速判断记忆系统是否包含相关内容
2. search_memory: RAG检索+rerank
3. commit_session: 提交会话信息进行归档和压缩 messages -> notes，异步任务


### 记忆管理系统后台任务
1. session处理队列: 将commit_session 摘要为notes -> 启动experience_agent, knowledge_agent, inner_agent，注入各自的system_prompt和notes为上下文，进行记忆整理 -> 等待所有代理运行成功 -> 触发 re_embed -> 更新状态 -> 启动experience_agent, knowledge_agent, inner_agent，注入各自的system_prompt和notes为上下文，进行记忆整理 -> 等待所有代理运行成功 -> 触发 re_embed -> 更新状态
2. 
## Design
### 技术栈

**记忆服务 (Python)**
| 组件 | 技术选型 | 说明 |
|------|----------|------|
| Web框架 | FastAPI | 异步HTTP服务，自动OpenAPI文档 |
| 向量数据库 | Chroma | 嵌入式向量库，支持持久化 |
| Embedding | 云接口调用 | 支持多provider切换 |
| Rerank | Cohere/云接口 | 检索结果重排序 |
| 异步任务 | asyncio + 任务队列 | Session压缩处理 |
| 文件监控 | watchdog | 检测记忆文件变更 |

**Agent运行时 (Go)**
| 组件 | 职责 |
|------|------|
| HTTP Client | 调用记忆服务API |
| 主线程 | 任务执行Agent |
| 记忆管理线程 | 记忆整理Agent |

### 数据流

**核心原则**: memory目录是Agent运行时与记忆服务的共享存储，Agent直接读写文件，记忆服务负责索引与压缩。

```
                    ┌─────────────────────────────┐
                    │        memory/ (共享)        │
                    │  ├─ experience/  ← 服务写入
                    │  ├─ knowledge/   ← 服务写入
                    │  ├─ notes/        ← 服务写入
                    │  ├─ histories/    ← 服务写入
                    │  └─ inner/        ← 自动装载(mmap) + 服务更新
                    └──────────┬──────────────────┘
                               │
        ┌──────────────────────┼──────────────────────┐
        │                      │                      │
        ▼                      ▼                      ▼
┌───────────────┐      ┌───────────────┐      ┌───────────────┐
│   主Agent     │      │ 监控goroutine │      │   记忆服务    │
│    (Go)       │      │    (Go)       │      │   (Python)    │
├───────────────┤      ├───────────────┤      ├───────────────┤
│ [inner自动装载]│      │               │      │               │
│               │      │ 轮询任务状态   │      │ 监控文件变更   │
│ HTTP: search  │      │ 失败时重试    │      │ 维护RAG索引   │
│ HTTP: commit  │      │               │      │ 异步任务:     │
│               │      │               │      │  - 压缩notes  │
│               │      │               │      │  - 多Agent整理 │
│               │      │               │      │  - re_embed   │
└───────────────┘      └───────────────┘      └───────────────┘
```

**Session生命周期**:

```
用户请求
    │
    ▼
┌───────────────┐    POST /trigger_memory  ┌─────────────────┐
│  主Agent (Go)  │ ───────────────────────▶ │                 │
│ [inner已装载]  │◀─────────────────────── │                 │
│               │    POST /search_memory   │                 │
│               │ ───────────────────────▶ │                 │
│               │◀─────────────────────── │                 │
└───────┬───────┘                          │   记忆服务      │
        │                                   │   (Python)      │
        │ Session结束                       │                 │
        ▼                                   │                 │
┌───────────────┐    POST /commit_session  │                 │
│               │ ───────────────────────▶ │  异步任务队列:  │
│               │   返回 task_id           │  ┌─────────────┐│
│               │                          │  │ 1. Write    ││
└───────┬───────┘                          │  │    histories││
        │                                   │  │ 2. 压缩     ││
        │ 启动监控goroutine                 │  │    → notes  ││
        ▼                                   │  │ 3. 并行启动 ││
┌───────────────┐   GET /task/{id}/status  │  │    3 Agent  ││
│ 监控goroutine │◀─────────────────────────│  │    整理     ││
│  (Go)         │                          │  │ 4. re_embed ││
│               │   失败时POST /retry      │  └─────────────┘│
│ - 轮询状态    │ ───────────────────────▶ │                 │
│ - 失败重试    │                          │                 │
└───────────────┘                          └─────────────────┘
```

**Python端异步任务详情**:

```
commit_session提交
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│                      异步任务处理器                           │
├─────────────────────────────────────────────────────────────┤
│  Step 1: Write histories/{session_id}.json                  │
│                                                             │
│  Step 2: LLM压缩 messages → notes/{session_id}.md           │
│                                                             │
│  Step 3: 并行启动记忆整理Agent                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │ experience  │  │ knowledge   │  │ inner       │         │
│  │ _agent      │  │ _agent      │  │ _agent      │         │
│  │             │  │             │  │             │         │
│  │ 提取经验规则 │  │ 提取事实知识 │  │ 更新自我/用户│         │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘         │
│         │                │                │                 │
│         └────────────────┼────────────────┘                 │
│                          ▼                                  │
│  Step 4: 汇总结果，写入 memory/experience/knowledge/inner   │
│                                                             │
│  Step 5: 触发 re_embed 更新向量索引                         │
│                                                             │
│  Step 6: 更新任务状态为 completed                           │
└─────────────────────────────────────────────────────────────┘
```

### 系统架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                      OxenCode Agent Runtime (Go)                    │
├────────────────────────────────┬────────────────────────────────────┤
│      主线程 (Task Agent)       │      监控goroutine                 │
│  ┌──────────────────────┐     │  ┌────────────────────────────┐    │
│  │ [inner 自动装载]       │     │  │                            │    │
│  │                      │     │  │ - 轮询任务状态              │    │
│  │ - 处理用户请求        │     │  │ - 失败时调用retry_session  │    │
│  │ - HTTP: search/trigger│     │  │                            │    │
│  │ - HTTP: commit_session│     │  └────────────────────────────┘    │
│  └──────────────────────┘     │                                    │
├────────────────────────────────┴────────────────────────────────────┤
│                          共享文件系统: memory/                       │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  experience/  knowledge/  notes/  histories/  inner/        │   │
│  └─────────────────────────────────────────────────────────────┘   │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ HTTP
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    Memory Service (Python FastAPI)                  │
├─────────────────────────────────────────────────────────────────────┤
│  API Layer (只读+任务)                                              │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  POST /trigger_memory  - 快速判断是否有相关记忆              │   │
│  │  POST /search_memory   - RAG检索+rerank                     │   │
│  │  POST /commit_session  - 提交session，异步处理              │   │
│  │  GET  /task/{id}/status - 查询异步任务状态                   │   │
│  │  GET  /notes/{id}      - 获取压缩后的notes                   │   │
│  │  POST /re_embed        - 增量重建索引                        │   │
│  │  POST /retry_session   - 重试失败的session                   │   │
│  └──────────────────────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────────────────────┤
│  Core Layer                                                         │
│  ┌─────────────────┐  ┌─────────────────┐  ┌───────────────────┐   │
│  │   文件监控       │  │   RAG 索引层    │  │   状态管理层     │   │
│  │                 │  │                 │  │                   │   │
│  │ watchdog监控    │  │ - Chroma DB     │  │ - 文件哈希表      │   │
│  │ memory/变更     │  │ - Embedding API │  │ - 索引状态表      │   │
│  │                 │  │ - Rerank API    │  │ - 任务状态表      │   │
│  └─────────────────┘  └─────────────────┘  └───────────────────┘   │
├─────────────────────────────────────────────────────────────────────┤
│  Async Task Queue                                                   │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  commit_session任务:                                          │   │
│  │  1. Write: histories/{session}.json                          │   │
│  │  2. 压缩: messages → notes/{session}.md (LLM)                │   │
│  │  3. 并行启动记忆整理Agent:                                    │   │
│  │     - experience_agent → memory/experience/                  │   │
│  │     - knowledge_agent  → memory/knowledge/                   │   │
│  │     - inner_agent      → memory/inner/                       │   │
│  │  4. re_embed 增量索引                                         │   │
│  │  5. 更新任务状态                                              │   │
│  └──────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
```

### API 接口定义

记忆服务暴露**查询**和**异步任务**接口。Agent不直接写入memory，整理工作由异步任务完成。

**对外接口**（Go端调用）：

```
POST /trigger_memory
Request:  { "query": "用户查询文本" }
Response: { "has_relevant": true, "hint": "有相关经验记录", "score": 0.85 }

POST /search_memory
Request:  { "queries": ["查询文本1", "查询文本2"], "top_k": 5, "types": ["experience", "knowledge"] }
Response: { "results": [{ "id": "xxx", "description": "描述内容", "score": 0.85, "excerpt": "..." }] }

POST /load_memory
Request:  { "ids": ["id1", "id2"] }
Response: { "memories": [{ "id": "xxx", "content": "完整内容", "source": "experience/xxx.md" }] }

POST /commit_session
Request:  { "session_id": "xxx", "messages": [...] }
Response: { "task_id": "task_xxx" }
# 提交session，异步执行：压缩 → 多Agent整理 → re_embed

GET /task/{task_id}/status
Response: { "status": "completed|pending|running|failed", "error_message": "..." }

POST /retry_session
Request:  { "session_id": "xxx" }
Response: { "task_id": "task_xxx" }
# 重试失败的session处理
```

**内部接口**（不对外暴露）：

```
GET /notes/{session_id}     # 异步任务内部使用，加载notes供多Agent整理
POST /re_embed              # 异步任务内部调用，增量更新向量索引
```

### 记忆分层

| 层级 | 目录 | 访问方式 | 特点 | RAG索引 |
|------|------|----------|------|---------|
| 热层 | inner/self.md, inner/user.md | 自动装载到上下文(mmap) | 高频必需，保持简洁 | ❌ |
| 温层 | experience, knowledge, notes | POST /search_memory | 有价值信息，按需检索 | ✅ |
| 冷层 | histories | Read工具直接读取 | 原始轨迹，低密度 | ❌ |

### 文件定义
#### 范围
- experience
- knowledge
- notes

#### schema
```markdown
---
description: 描述文档的主要内容/调用条件
---
正文部分
```

#### 约束
- 每个文件主题清晰，一个主题一个文件
- 单个文件大小有硬约束，超过长度会被截断


### 实现顺序

#### Phase 1: 记忆服务基础 (Python)
1. FastAPI项目骨架
2. 目录结构与文件监控(watchdog)
3. 元数据管理（哈希、状态）

**验收标准：**
- [x] `POST /health` 返回 `{"status": "ok"}`
- [x] memory目录结构正确创建：inner/, experience/, knowledge/, notes/, histories/
- [x] 文件变更检测：新建/修改/删除文件能触发事件并记录到元数据
- [x] 元数据持久化：重启服务后能恢复文件哈希和索引状态
- [x] 配置文件支持：端口、memory路径、embedding provider可配置

#### Phase 2: RAG索引层 (Python)
1. Chroma集成
2. Embedding接口（多provider支持）
3. `/re_embed` 增量索引
4. `/search_memory` 检索，返回元数据
5. `/trigger_memory` 快速判断
6. `/load_memory` 全量加载

**验收标准：**
- [x] `POST /re_embed` 能检测文件变更，仅索引新增/修改的文件
- [x] 未变更文件不重复索引（哈希校验通过）
- [x] `POST /search_memory` 多query查询返回正确的id、description、score
- [x] `POST /trigger_memory` 能返回是否命中相关记忆（boolean）
- [x] `POST /load_memory` 能批量加载指定id的记忆文件内容
- [x] 支持至少2种embedding provider（Qwen、Mock）
- [x] 删除文件后索引能正确清理

#### Phase 3: 异步任务层 (Python)
1. asyncio任务队列
2. `/commit_session` 接口
3. 写入histories + messages→notes压缩（LLM调用）
4. 任务状态管理
5. `/task/{id}/status` 和 `/notes/{id}` 接口
6. `/retry_session` 重试接口

**验收标准：**
- [x] messages正确写入histories/{session_id}.json
- [x] `POST /commit_session` 写入成功后，立即返回task_id，不阻塞
- [x] LLM压缩生成notes/{session_id}.md，包含正确frontmatter
- [x] `GET /task/{id}/status` 正确返回pending/completed/failed状态
- [x] 任务完成后 `GET /notes/{session_id}` 返回压缩内容
- [x] 任务失败时状态包含错误信息
- [x] 服务重启后能恢复未完成任务状态
- [x] `POST /retry_session` 能对已存在histories但处理失败的session重新处理
- [x] retry不重复写入histories，只重新执行压缩

#### Phase 4: Go端集成
1. 记忆服务HTTP客户端封装为go sdk
2. inner目录自动装载机制（mmap式）
3. 主线程：trigger_memory, search_memory调用
4. Session结束：调用commit_session

**验收标准：**
- [x] Go客户端能成功调用所有记忆服务API
- [x] inner/self.md 和 inner/user.md 内容自动注入Agent上下文
- [x] Agent能通过trigger_memory快速判断相关记忆
- [x] Agent能通过search_memory工具获取相关记忆description
- [x] Agent能通过load_memory工具获取完整记忆内容
- [x] /commit-memory命令能提交session并返回task_id
- [x] 网络错误时有明确的错误日志和重试机制

#### Phase 5: 架构变更处理
原设计Agent自主管理记忆，现变更为Memory System统一管理。本阶段处理架构调整。

**变更内容**：
1. ~~Go端记忆管理Agent~~ → 不再需要，整理权交给Python端
2. Go端新增轻量级监控goroutine：提交session后轮询状态、失败重试
3. inner目录装载机制已实现，确认正常工作
4. Python端删除内部API：`/notes/{id}`和`/re_embed`（内部直接调用函数）

**任务清单**：
- [x] 确认inner目录自动装载机制（loadInnerVars）
- [x] 确认commit-session命令可用
- [x] Python端：删除`/notes/{session_id}`和`/re_embed` REST API端点
- [x] Go端：移除GetNotes和ReEmbed客户端方法，添加RetrySession方法
- [x] 实现监控goroutine
  - [x] commit_session返回后启动goroutine
  - [x] 轮询GET /task/{id}/status
  - [x] 状态为failed时调用POST /retry_session
  - [x] 最大重试次数可配置（config.toml）
- [x] 添加监控配置项到config.go和config.example.toml
- [x] 更新相关文档和注释

**验收标准**：
- [x] inner/self.md和inner/user.md自动注入Agent上下文
- [x] /commit-memory命令可提交session并返回task_id
- [x] Python端对外API仅保留：trigger/search/load/commit/task/status/retry
- [x] 监控goroutine能正确轮询任务状态
- [x] 任务失败时能自动重试（默认最多3次，可配置）
- [x] 有完整的日志记录

#### Phase 6: 多Agent记忆整理 (Python)
1. 设计experience_agent、knowledge_agent、inner_agent系统提示
2. 实现Agent基类与LLM调用封装
3. 集成到异步任务流程：notes压缩完成后启动多Agent
4. 并发执行多Agent，收集整理结果
5. 汇总写入experience/knowledge/inner目录
6. 自动触发re_embed

**验收标准：**
- [ ] experience_agent能从notes提取经验规则（"遇到X情况，应该Y"）
- [ ] knowledge_agent能从notes提取事实知识（陈述性信息）
- [ ] inner_agent能更新self.md和user.md（自我认知、用户偏好）
- [ ] 三个Agent并行执行，结果汇总后统一写入
- [ ] 写入文件符合schema规范（frontmatter + 正文）
- [ ] 整理完成后自动触发re_embed更新索引
- [ ] Agent执行失败不影响notes压缩结果
- [ ] 完整的日志记录和错误处理

#### Phase 7: 优化与测试
1. Rerank集成
2. 记忆去重与合并
3. 性能测试
4. 部署配置

**验收标准：**
- [ ] search_memory支持rerank，提升检索精度
- [ ] 相似记忆能自动去重或提示合并
- [ ] 1000条记忆检索延迟 < 500ms
- [ ] commit_session压缩延迟 < 30s（100条消息）
- [ ] Docker部署配置完整
- [ ] 端到端测试：完整session生命周期 → 记忆生成 → 检索验证
- [ ] 压力测试：并发10个commit_session无阻塞

## 项目状态
- [x] Phase 1: 记忆服务基础 (Python)
- [x] Phase 2: RAG索引层 (Python)
- [x] Phase 3: 异步任务层 (Python)
- [x] Phase 4: Go端集成
- [x] Phase 5: 架构变更处理（监控goroutine）
- [ ] Phase 6: 多Agent记忆整理 (Python)
- [ ] Phase 7: 优化与测试
