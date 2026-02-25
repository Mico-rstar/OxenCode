# Why
当前OxenCode不具备运行长程任务的能力，当上下文增长超出模型限制后会陷入错误。为了让OxenCode具有执行长程任务能力，故引入上下文管理系统

# Basic idea
- 通过Session隔离任务上下文
- 分批异步压缩，而非上下文达到阈值后再全量压缩
- 上下文分级，压缩程度 L0 > L1 > L2
- 对工具进行针对性优化，例如Read工具读取超大文件时应该阻止，允许agent按行读取
- 将历史对话进行压缩放入上下文，在上下文中保留原始信息引用

# Key requirements
- 前缀稳定，保证较高的缓存命中率
- 无明显的上下文切换卡顿
- 上下文切换前后Agent performance保持稳定
- 数小时的长程任务上下文不会腐烂

# Concept
- System Prompt: OxenCode的System prompt采用模块化机制，main prompt用于构建系统提示词框架，可以通过占位符引入prompt module进行灵活的提示词构建。System prompt主要包含: 身份定义，边界，能力，tool definition，skill元数据（未实现），mcp tool（未实现）， soul（未实现）， user（未实现）等
- Session: 用于隔离上下文窗口
- Page: 这是OxenCode中的特殊概念，Page本质上维护一轮交互产生的所有message的对象，Page可以预定义schema用于定义压缩的格式，Page会将messages按照schema定义进行压缩提取后渲染在上下文中，而原始的messages会被卸载出上下文，Page保留messages的引用，并允许agent使用read，grep工具查询原始messages。Page渲染后相当于单条message，包含压缩信息以及引用文件路径
- message: 上下文系统最小单位，包括用户输出，工具调用，assitant回答，系统提示词等信息
- LX: L2代表原始message集合，L1代表被压缩为Page粒度，L0本质上只有1个Page，是将多个Page压缩为一个Page得到的特殊的Page

# Core struct
注意，以下只写出关键字段和方法，实际设计应该具体情况具体分析
## Page
```go
// Page 维护一轮交互的所有 message
type Page struct {
    ID           PageID
	// 压缩策略
	Strategy    *CompressionStrategy `json:"strategy"`   // 压缩策略配置
	// 内容缓存
	Content     string            `json:"content"`      // 根据 schema 压缩后的内容缓存
	// 原始消息归档
	ArchivedFile    string        `json:"archived_file"` // 归档文件路径
    // 标识类型
    Type           PageType        `json:"type"`         // 标识 L0 L1 L2         
}


```

## CompressedStrategy
```go
// CompressionStrategy 压缩策略
type CompressionStrategy struct {
	MaxCompressionRate float64       `json:"max_compression_rate"` // 最大压缩率
	MinCompressionRate float64       `json:"min_compression_rate"` // 最小压缩率
	Schema             string        `json:"schema"`               // 压缩使用的 schema 模板
    Skill              string        `json:"skill"`                // 压缩模型具有通用的系统提示词，skill可以用于为特定压缩任务定义工作流和few-shot示例
	CompressionModel   string        `json:"compression_model"`    // 用于压缩的模型标识
	Timeout            time.Duration `json:"timeout"`              // 压缩超时时间
}
```
如果没有配置 CompressionModel，那么不进行任何压缩，用于L2级Page的生成


## session
```go
// Session 上下文会话，管理完整的上下文窗口
type Session struct {

	// 上下文窗口 (system -> L0 -> L1 -> L2)
	SystemPrompt    string      `json:"system_prompt"`    // 系统 Prompt（缓存）
	L0Page          PageID       `json:"l0_page"`          // 全局唯一的 L0 Page
	L1Pages         []PageID     `json:"l1_pages"`         // L1 Pages 列表（按时间倒序）
	L2Pages         []PageID     `json:"l2_pages"`         // L2 Pages 列表（按时间倒序）

	// 配置
	MaxL1Pages      int         `json:"max_l1_pages"`     // L1 Page 最大数量
	L1Strategy      *CompressionStrategy `json:"l1_strategy"`
	L2Strategy      *CompressionStrategy `json:"l2_strategy"`
	L0Strategy      *CompressionStrategy `json:"l0_strategy"`

	// 异步压缩管理
	compressor      *Compressor `json:"-"`               // 压缩器
	compressQueue   chan *Page  `json:"-"`               // 压缩任务队列
}
```

## Compressor
```go
type Compressor interface {
    Compress(ctx context.Context, raw string, strategy *CompressionStrategy) (output string) // 需要有重试机制，确保满足CompressionStrategy的限制

}
```




# How
## Context window overview
system prompt -> L0 -> L1 -> L2
L0: 只有全局唯一的一个Page，是对比较久远的Page的高层次压缩，保留Page引用
L1: 对原始messages的轻度压缩，确保较高召回率，保留原始messages的引用。上下文中存在多个L1级别Page，每轮交互产生一个L1 Page
L2: 仍在进行的交互，或者系统尚未处理的messages，按照一轮交互的粒度组织成一个个块，但不进行其他处理，等待压缩为Page


## Transform strategy
### L2 -> L1
#### Given
- 一轮交互完成后，调用commit提交到处理队列

#### How
- 通过 L1 Page定义的schema进行压缩，将原始messages归档到文件系统

#### Common cases 
下一轮会话到来时，Page生成队列为空，此时只渲染Page，而卸载原始messages

#### Boundary cases
1. 下一轮会话到来时，Page生成队列依然没有处理完成
   - 将原始messages写入上下文，等待下一轮交互再更新


### L0 -> L1
#### Given
- L1 Page达到容量上限，自动压缩前n Pages

#### How
- 将old L0 page.content + n L1 pages.content 拼接作为输入，通过L0 schema生成new L0 page；L0 Page的archivedFile追加n L1 Pages的archivedFile的内容到末尾 
- schema定义可能的角度：核心概念提取，检索关键词，摘要信息,...

#### Common cases 
达到Given条件，创建压缩goroutine，使用L0定义的schema压缩，下一轮对话发起时，经过检查点发现L0更新，重新构造上下文

#### Boundary cases
1. 压缩goroutine超时
2. 上下文增长速度超过压缩goroutine的处理速度


# schema & strategy definition
## L1
对一轮交互进行的摘要，应该保留用户给的任务是什么，用户意图是什么，agent做了什么事，工具执行结果和关键返回，任务完成的状态等信息，而对于详细的工具执行输出日志这类低密度信息要简要舍弃
### schema 
```markdown
# user input
用户的原始输入是什么，用户意图是什么

# Agent Actions
Agent 执行了哪些操作（工具调用），每个操作的关键结果是什么
- 工具名称: 简要描述
- 工具名称: 简要描述
- Agent输出摘要

# Task Status
当前任务的完成状态是什么（进行中/已完成/遇到问题）

# Key Information
这轮对话中的关键信息（代码片段、配置、重要发现、错误、矛盾等）
```


## L0
L0 Page是对历史page的高层次概括，应该明确过去做了用户问了什么，agent做了什么，项目背景，关键代码片段，对项目结构的理解，代码风格等高层次信息
### schema 
```markdown
# Project Context
项目背景、技术栈、项目结构等

# User Inputs
- input1
- input2
- ...

# Key Achievements
已完成的主要工作

# Key Information
关键代码位置和片段、关键发现、矛盾错误

# Core concepts
核心概念、关键词以及简短的解释


# Important Notes
重要的约束条件、配置、代码风格等
```
