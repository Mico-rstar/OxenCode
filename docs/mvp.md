# OxenCode MVP 设计计划

## MVP 目标

构建一个最小可行产品，验证核心Agent工程技术，包括流式输出、工具调用、权限系统和多轮对话。

## 核心功能范围

### Phase 1: 基础框架 (Week 1)

#### 1.1 项目初始化
- [x] 目录结构搭建
  ```
  oxencode/
  ├── cmd/
  │   └── oxencode/
  │       └── main.go
  ├── internal/
  │   ├── agent/          # Agent核心逻辑
  │   ├── ui/             # TUI界面
  │   ├── tools/          # 工具实现
  │   ├── config/         # 配置管理
  │   └── history/        # 历史记录
  ├── pkg/
  │   └── api/            # API客户端
  └── docs/
  ```
- [x] 依赖集成
  - Bubble Tea (TUI框架)
  - fantasy (AI SDK)
  - lipgloss (样式)
  - Viper (配置管理)

#### 1.2 基础TUI框架
- [x] Bubble Tea 基础模型 (Model)
- [x] 输入框组件 (用户输入)
- [x] 消息显示区域
- [x] 基础事件处理 (键盘输入)

**🎯 里程碑 M1.1 - 基础界面**
- 可编译运行的TUI应用
- 输入框 + 消息显示区域
- Enter键提交消息

---

### Phase 2: AI对话核心 (Week 2)

#### 2.1 LLM集成
- [x] fantasy SDK集成
- [x] 多 Provider 支持 (Anthropic, OpenAI, Azure, Bedrock, Google, OpenRouter, Vercel, OpenAICompat, Qwen, DeepSeek, GLM)
- [x] 基础对话功能 (单轮)
- [x] 配置文件支持 (config.example.toml)
- [x] 环境变量支持 (ANTHROPIC_API_KEY, OPENAI_API_KEY, DASHSCOPE_API_KEY, DEEPSEEK_API_KEY, ZHIPU_API_KEY, etc.)

#### 2.2 流式输出
- [x] SSE流式响应处理
- [x] 打字机效果实现
- [x] 实时UI更新
- [x] 流式数据缓冲与渲染

#### 2.3 多轮对话
- [x] 对话历史存储
- [x] Context管理
- [x] 消息序列化 (保存/加载)

**🎯 里程碑 M1.2 - 可对话的Agent**
- 连接API，实现单轮对话
- 流式输出（打字机效果）
- 多轮对话（记住上下文）

---

### Phase 3: 工具系统 (Week 3-4)

#### 3.1 工具调用框架
- [ ] 工具定义与注册
- [ ] Tool Schema生成
- [ ] 函数调用解析

#### 3.2 核心工具实现

**优先级 P0 (MVP必需):**
- [ ] **Glob** - 文件模式匹配
  - 支持通配符模式
  - 返回匹配文件列表
- [ ] **Grep** - 内容搜索
  - 正则表达式支持
  - 文件类型过滤
- [ ] **Read** - 文件读取
  - 读取指定文件内容
  - 支持大文件分页

**优先级 P1 (重要功能):**
- [ ] **Bash** - 命令执行
  - 异步执行
  - 输出捕获
- [ ] **Write** - 文件写入
  - 创建新文件
  - 覆盖模式
- [ ] **Edit** - 文件编辑
  - 字符串替换
  - 支持多处替换

#### 3.3 ReAct循环
- [ ] Thought-Action-Observation循环
- [ ] 工具结果反馈
- [ ] 自动迭代决策
- [ ] 完成条件判断

**🎯 里程碑 M2 - 可执行的Agent**
- 工具调用框架完成
- P0工具可用（Glob/Grep/Read）
- ReAct循环自主运行

---

### Phase 4: 权限系统 (Week 5)

#### 4.1 权限框架
- [ ] 危险操作定义
- [ ] 权限检查钩子
- [ ] 用户确认UI

#### 4.2 授权策略
- [ ] 一次性授权
- [ ] 持久化授权 (配置文件)
- [ ] 授权撤销

**危险操作列表:**
- Bash: 删除操作 (`rm`, `rmdir`)
- Bash: 系统修改 (`sudo`, `chmod`)
- Write/Edit: 覆盖重要文件 (`.env`, `credentials`)
- Git: 破坏性操作 (`git reset --hard`, `git clean -f`)

**🎯 里程碑 M3.1 - 安全可控**
- 权限检查生效
- 危险操作需用户确认
- 授权配置持久化

---

### Phase 5: 取消与状态管理 (Week 6)

#### 5.1 取消机制
- [ ] Esc键监听
- [ ] Context取消传播
- [ ] LLM流中断
- [ ] 工具执行中断
- [ ] 状态清理

#### 5.2 状态持久化
- [ ] 对话历史保存
- [ ] 会话恢复
- [ ] 缓存管理

**🎯 里程碑 M3.2 - 完整MVP**
- Esc键中断任何操作
- 对话历史保存/恢复
- 完整的验收测试通过

---

## 技术决策记录

### 决策
| 决策项 | 选择 | 理由 |
|--------|------|------|
| 语言 | Go | 学习目的，生态成熟 |
| AI SDK | fantasy | Claude官方SDK |
| TUI框架 | Bubble Tea | Elm架构，适合复杂交互 |
| 配置格式 | TOML | 简洁清晰，更适合Go项目 |
| 配置管理 | Viper | 成熟方案，支持多种格式 |
| 对话历史 | JSON文件 | 轻量级，易于调试 |


## 验收标准

### MVP完成后，系统应能够:

1. **完成一个完整的编程任务**
   - 用户: "帮我找出所有.go文件中包含'error'的行"
   - Agent: 使用Glob找文件 → Grep搜索内容 → 返回结果

2. **安全地执行文件操作**
   - 用户: "创建test.txt并写入内容"
   - Agent: 检查权限 → 执行Write → 确认成功

3. **支持中断恢复**
   - 用户在任务进行中按Esc
   - 系统立即停止并保存当前状态

4. **维护对话上下文**
   - 用户: "刚才找到的文件，读取第一个"
   - Agent: 理解"第一个"指代上轮结果



## 执行计划
Phase 1 (基础框架) ←─ 所有阶段的基础
    ↓
Phase 2 (AI对话核心) ←─ 工具和状态管理的基础
    ↓
├── Phase 3 (工具系统) ←──┐
└── Phase 4 (权限系统) ←──┤ (可并行)
    ↓                      ↓
    └───── Phase 5 (取消与状态管理)