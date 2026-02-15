## 概述
OxenCode是一个学习型的CodeAgent项目，用于让我深入理解context engineering, function calling, ReAct等一系列Agent工程化的核心技术

OxenCode在功能设计上主要参考claude code，核心功能包括：
- 流式输出 - 打字机效果，实时显示 AI 思考过程
- 工具调用 - Glob/Grep/View/Bash/Write/Edit 六大工具
- 权限系统 - 危险操作确认，支持持久授权
- 多轮对话 - 完整的上下文记忆
- 取消支持 - Esc 随时中断，不浪费 Token

技术栈：
- 语言：Go 
- ai sdk：fantasy
- TUI框架：Bubble Tea