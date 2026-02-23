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
- 当L1 Page总数大于 n时，由小模型判断当前任务和前n pages相关性较低，自主触发压缩
**注意：n pages也可以改为token阈值**

#### How
- 通过L0定义的schema压缩n pages，将压缩后的信息**追加**到原来的 L0 Page中
- schema定义可能的角度：核心概念提取，检索关键词，摘要信息,...

#### Common cases 
达到Given条件，创建压缩goroutine，使用L0定义的schema压缩，下一轮对话发起时，经过检查点发现L0更新，重新构造上下文

#### Boundary cases
1. 压缩goroutine超时
2. 上下文增长速度超过压缩goroutine的处理速度