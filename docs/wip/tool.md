# OxenCode 工具系统设计

## 1. 概述

工具系统是 OxenCode Agent 的核心能力，使 AI 能够通过调用外部工具完成复杂任务。本设计基于 ReAct (Reasoning + Acting) 范式，实现 Thought-Action-Observation 循环。

### 1.1 设计目标

- **简洁性**: 工具定义简单，易于扩展
- **类型安全**: 参数校验和类型转换自动化
- **可组合**: 工具可以调用其他工具
- **可观测**: 每个工具调用都可追踪和记录
- **安全性**: 支持权限检查和危险操作拦截

### 1.2 架构概览

```
┌─────────────────────────────────────────────────────────────┐
│                         UI Layer                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Messages   │  │   Input      │  │  Permission  │      │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │
└─────────┼──────────────────┼──────────────────┼─────────────┘
          │                  │                  │
          ▼                  ▼                  ▼
┌─────────────────────────────────────────────────────────────┐
│                      Agent Layer                             │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              ReAct Loop Controller                    │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────────────┐   │   │
│  │  │ Thought  │→│  Action  │→│   Observation     │   │   │
│  │  └──────────┘  └────┬─────┘  └──────────────────┘   │   │
│  │                      │                                │   │
│  │              ┌───────▼────────┐                       │   │
│  │              │  Tool Manager  │                       │   │
│  │              └───────┬────────┘                       │   │
│  └──────────────────────┼───────────────────────────────┘   │
└──────────────────────────┼───────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                      Tool Layer                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │  Glob    │  │  Grep    │  │  Read    │  │  Write   │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                  │
│  │  Edit    │  │  Bash    │  │  ...     │                  │
│  └──────────┘  └──────────┘  └──────────┘                  │
└──────────────────────────────┬──────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────┐
│                  Execution Environment Layer                │
│  ┌─────────────────────────────────────────────────────┐   │
│  │           Environment Interface                      │   │
│  │  - ReadFile()  - WriteFile()  - ListFiles()         │   │
│  │  - ExecCommand()  - GetWorkingDir()                 │   │
│  └─────────────────────────┬───────────────────────────┘   │
│                            │                                │
│  ┌─────────────────┐  ┌───▼─────────────┐                  │
│  │ Local FS (MVP)  │  │ Container (v2)  │                  │
│  └─────────────────┘  └─────────────────┘                  │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. 执行环境 (Execution Environment)

### 2.1 环境抽象接口

执行环境抽象是工具系统的关键设计，使得工具与具体的运行环境解耦，支持未来无缝切换到沙箱或容器环境。

```go
// internal/tools/env.go

package tools

import (
    "context"
    "io/fs"
)

// Environment 工具执行环境接口
type Environment interface {
    // GetWorkingDirectory 获取当前工作目录
    GetWorkingDirectory() string

    // ReadFile 读取文件内容
    ReadFile(path string) ([]byte, error)

    // WriteFile 写入文件内容
    WriteFile(path string, data []byte, perm fs.FileMode) error

    // ListFiles 列出文件（支持通配符模式）
    ListFiles(pattern string) ([]string, error)

    // FileExists 检查文件是否存在
    FileExists(path string) bool

    // ExecCommand 执行命令
    ExecCommand(ctx context.Context, cmd string, args ...string) ([]byte, error)

    // ExecCommandWithWorkingDir 在指定目录执行命令
    ExecCommandWithWorkingDir(ctx context.Context, dir, cmd string, args ...string) ([]byte, error)

    // ResolvePath 解析相对路径为绝对路径
    ResolvePath(path string) string

    // Cleanup 清理资源（环境销毁时调用）
    Cleanup() error
}
```

### 2.2 本地文件系统环境 (MVP 实现)

```go
// internal/tools/env_local.go

package tools

import (
    "context"
    "io/fs"
    "os"
    "os/exec"
    "path/filepath"
)

// LocalEnvironment 本地文件系统环境实现
type LocalEnvironment struct {
    basePath string // 工作根目录
}

// NewLocalEnvironment 创建本地环境
func NewLocalEnvironment(basePath string) (*LocalEnvironment, error) {
    // 转换为绝对路径
    absPath, err := filepath.Abs(basePath)
    if err != nil {
        return nil, err
    }

    // 确保目录存在
    if err := os.MkdirAll(absPath, 0755); err != nil {
        return nil, err
    }

    return &LocalEnvironment{
        basePath: absPath,
    }, nil
}

func (e *LocalEnvironment) GetWorkingDirectory() string {
    return e.basePath
}

func (e *LocalEnvironment) ReadFile(path string) ([]byte, error) {
    fullPath := e.ResolvePath(path)
    return os.ReadFile(fullPath)
}

func (e *LocalEnvironment) WriteFile(path string, data []byte, perm fs.FileMode) error {
    fullPath := e.ResolvePath(path)

    // 确保父目录存在
    dir := filepath.Dir(fullPath)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return err
    }

    return os.WriteFile(fullPath, data, perm)
}

func (e *LocalEnvironment) ListFiles(pattern string) ([]string, error) {
    fullPath := e.ResolvePath(pattern)
    return filepath.Glob(fullPath)
}

func (e *LocalEnvironment) FileExists(path string) bool {
    fullPath := e.ResolvePath(path)
    _, err := os.Stat(fullPath)
    return err == nil
}

func (e *LocalEnvironment) ExecCommand(ctx context.Context, cmd string, args ...string) ([]byte, error) {
    c := exec.CommandContext(ctx, cmd, args...)
    c.Dir = e.basePath
    return c.CombinedOutput()
}

func (e *LocalEnvironment) ExecCommandWithWorkingDir(ctx context.Context, dir, cmd string, args ...string) ([]byte, error) {
    fullPath := e.ResolvePath(dir)
    c := exec.CommandContext(ctx, cmd, args...)
    c.Dir = fullPath
    return c.CombinedOutput()
}

func (e *LocalEnvironment) ResolvePath(path string) string {
    if filepath.IsAbs(path) {
        return path
    }
    return filepath.Join(e.basePath, path)
}

func (e *LocalEnvironment) Cleanup() error {
    // 本地环境无需清理
    return nil
}
```

### 2.3 容器环境 (Future v2)

```go
// internal/tools/env_container.go

package tools

import (
    "context"
    "docker.io/docker/client"
)

// ContainerEnvironment 容器环境实现（未来版本）
type ContainerEnvironment struct {
    cli         *client.Client
    containerID string
    workDir     string
}

// NewContainerEnvironment 创建容器环境
func NewContainerEnvironment(image string) (*ContainerEnvironment, error) {
    cli, err := client.NewClientWithOpts(client.FromEnv)
    if err != nil {
        return nil, err
    }

    // 创建并启动容器
    // ... 容器创建逻辑 ...

    return &ContainerEnvironment{
        cli:     cli,
        workDir: "/workspace",
    }, nil
}

func (e *ContainerEnvironment) GetWorkingDirectory() string {
    return e.workDir
}

func (e *ContainerEnvironment) ReadFile(path string) ([]byte, error) {
    // 使用 docker cp 从容器中复制文件
    // ...
    return nil, nil
}

func (e *ContainerEnvironment) WriteFile(path string, data []byte, perm fs.FileMode) error {
    // 使用 docker cp 将文件复制到容器中
    // ...
    return nil
}

// ... 其他方法实现 ...

func (e *ContainerEnvironment) Cleanup() error {
    // 停止并删除容器
    return e.cli.ContainerRemove(context.Background(), e.containerID, container.RemoveOptions{
        Force: true,
    })
}
```

### 2.4 工具使用环境

工具不再直接访问文件系统，而是通过 `Environment` 接口操作：

```go
// 工具持有环境引用
type ReadTool struct {
    env Environment
}

func NewReadTool(env Environment) *ReadTool {
    return &ReadTool{env: env}
}

func (t *ReadTool) Execute(ctx context.Context, input map[string]any) (string, error) {
    filePath := input["file_path"].(string)

    // 使用环境读取文件，而不是直接调用 os.ReadFile
    content, err := t.env.ReadFile(filePath)
    if err != nil {
        return "", fmt.Errorf("read failed: %w", err)
    }

    return string(content), nil
}
```

### 2.5 环境创建与生命周期

```go
// internal/agent/agent.go

type Agent struct {
    agent   fantasy.Agent
    config  *config.Config
    history []message.Message
    tools   *tools.Registry
    env     tools.Environment // 执行环境
}

func NewAgent(cfg *config.Config) (*Agent, error) {
    // ... 现有代码 ...

    // 创建执行环境（MVP 版本使用本地环境）
    env, err := tools.NewLocalEnvironment(cfg.WorkDir)
    if err != nil {
        return nil, fmt.Errorf("failed to create environment: %w", err)
    }

    // 创建工具注册表，并注入环境
    registry := tools.NewRegistry()
    registry.Register(tools.NewGlobTool(env))
    registry.Register(tools.NewGrepTool(env))
    registry.Register(tools.NewReadTool(env))
    registry.Register(tools.NewBashTool(env))
    // ...

    return &Agent{
        agent:   agent,
        config:  cfg,
        history: history,
        tools:   registry,
        env:     env,
    }, nil
}

func (a *Agent) Close() error {
    // 清理环境资源
    if a.env != nil {
        return a.env.Cleanup()
    }
    return nil
}
```

### 2.6 配置支持

在配置中添加工作目录和环境类型：

```go
// internal/config/config.go

type Config struct {
    // ... 现有字段 ...

    WorkDir       string `yaml:"work_dir" env:"OXEN_WORK_DIR" default:"."`
    EnvType       string `yaml:"env_type" env:"OXEN_ENV_TYPE" default:"local"` // local | container
    ContainerImage string `yaml:"container_image" env:"OXEN_CONTAINER_IMAGE"`
}
```

### 2.7 优势

1. **解耦**: 工具代码不依赖具体的文件系统实现
2. **可测试**: 可以轻松创建 Mock 环境进行单元测试
3. **安全**: 未来可以切换到隔离的容器环境执行工具
4. **灵活性**: 支持远程环境、临时环境等多种场景
5. **资源管理**: 通过 `Cleanup()` 方法统一管理环境生命周期

---

## 3. 核心数据结构

### 2.1 工具定义 (Tool Definition)

每个工具需要实现统一的接口：

```go
// internal/tools/tool.go

package tools

import (
    "context"
    "encoding/json"
)

// Tool 工具接口
type Tool interface {
    // Name 返回工具名称（唯一标识）
    Name() string

    // Description 返回工具描述（用于生成 schema）
    Description() string

    // Parameters 返回参数 schema (JSON Schema 格式)
    Parameters() json.RawMessage

    // Execute 执行工具
    Execute(ctx context.Context, input map[string]any) (string, error)

    // Validate 验证输入参数（可选，默认使用 schema 验证）
    Validate(input map[string]any) error
}

// ToolExecutor 工具执行器
type ToolExecutor struct {
    tools map[string]Tool
    // 权限检查器（Phase 4 实现）
    permissionChecker PermissionChecker
}

// PermissionChecker 权限检查接口
type PermissionChecker interface {
    Check(toolName string, input map[string]any) (bool, error)
    IsDangerous(toolName string, input map[string]any) bool
}
```

### 2.2 工具 Schema 格式

工具使用 JSON Schema 定义参数：

```json
{
  "name": "grep",
  "description": "在文件中搜索匹配正则表达式的内容",
  "input_schema": {
    "type": "object",
    "properties": {
      "pattern": {
        "type": "string",
        "description": "要搜索的正则表达式模式"
      },
      "path": {
        "type": "string",
        "description": "搜索的路径，默认为当前目录"
      },
      "file_pattern": {
        "type": "string",
        "description": "文件匹配模式（如 *.go）"
      }
    },
    "required": ["pattern"]
  }
}
```

### 2.3 工具调用流程

```
1. LLM 生成函数调用
   ↓
2. Agent 解析工具名称和参数
   ↓
3. ToolExecutor 执行前检查
   - 参数验证
   - 权限检查
   ↓
4. 执行工具
   ↓
5. 返回结果
   ↓
6. 将结果添加到 ReActLoop
   ↓
7. 继续下一轮循环
```

---

## 4. P0 工具实现

### 4.1 Glob 工具

文件路径模式匹配工具。

```go
// internal/tools/glob.go

type GlobTool struct {
    env Environment
}

func NewGlobTool(env Environment) *GlobTool {
    return &GlobTool{env: env}
}

func (t *GlobTool) Name() string {
    return "glob"
}

func (t *GlobTool) Description() string {
    return "使用通配符模式查找文件"
}

func (t *GlobTool) Parameters() json.RawMessage {
    return json.RawMessage(`{
        "type": "object",
        "properties": {
            "pattern": {
                "type": "string",
                "description": "文件匹配模式（支持 *, **, ? 等通配符）"
            },
            "path": {
                "type": "string",
                "description": "搜索路径，默认为当前目录"
            }
        },
        "required": ["pattern"]
    }`)
}

func (t *GlobTool) Execute(ctx context.Context, input map[string]any) (string, error) {
    pattern := input["pattern"].(string)
    path := "."
    if p, ok := input["path"].(string); ok {
        path = p
    }

    // 使用环境进行列表操作
    searchPattern := filepath.Join(path, pattern)
    matches, err := t.env.ListFiles(searchPattern)
    if err != nil {
        return "", fmt.Errorf("glob failed: %w", err)
    }

    // 格式化输出（相对于工作目录）
    result := make([]string, len(matches))
    workDir := t.env.GetWorkingDirectory()
    for i, m := range matches {
        relPath, _ := filepath.Rel(workDir, m)
        result[i] = relPath
    }

    return strings.Join(result, "\n"), nil
}
```

### 4.2 Grep 工具

内容搜索工具。

```go
// internal/tools/grep.go

type GrepTool struct {
    env Environment
}

func NewGrepTool(env Environment) *GrepTool {
    return &GrepTool{env: env}
}

func (t *GrepTool) Name() string {
    return "grep"
}

func (t *GrepTool) Description() string {
    return "在文件中搜索匹配正则表达式的内容"
}

func (t *GrepTool) Parameters() json.RawMessage {
    return json.RawMessage(`{
        "type": "object",
        "properties": {
            "pattern": {
                "type": "string",
                "description": "正则表达式搜索模式"
            },
            "path": {
                "type": "string",
                "description": "搜索路径，默认为当前目录"
            },
            "file_pattern": {
                "type": "string",
                "description": "文件过滤模式（如 *.go）"
            },
            "ignore_case": {
                "type": "boolean",
                "description": "忽略大小写"
            }
        },
        "required": ["pattern"]
    }`)
}

func (t *GrepTool) Execute(ctx context.Context, input map[string]any) (string, error) {
    pattern := input["pattern"].(string)
    path := "."
    ignoreCase := false

    if p, ok := input["path"].(string); ok {
        path = p
    }
    if ic, ok := input["ignore_case"].(bool); ok {
        ignoreCase = ic
    }

    // 编译正则表达式
    if ignoreCase {
        pattern = "(?i)" + pattern
    }
    regex, err := regexp.Compile(pattern)
    if err != nil {
        return "", fmt.Errorf("invalid regex: %w", err)
    }

    // 收集文件（使用环境）
    var files []string
    if fp, ok := input["file_pattern"].(string); ok {
        files, _ = t.env.ListFiles(filepath.Join(path, fp))
    } else {
        files, _ = t.env.ListFiles(filepath.Join(path, "*"))
    }

    workDir := t.env.GetWorkingDirectory()

    // 搜索匹配
    var results []string
    for _, file := range files {
        content, err := t.env.ReadFile(file)
        if err != nil {
            continue
        }

        relPath, _ := filepath.Rel(workDir, file)
        lines := strings.Split(string(content), "\n")
        for i, line := range lines {
            if regex.MatchString(line) {
                results = append(results, fmt.Sprintf("%s:%d:%s", relPath, i+1, line))
            }
        }
    }

    if len(results) == 0 {
        return "No matches found", nil
    }

    return strings.Join(results, "\n"), nil
}
```

### 4.3 Read 工具

文件读取工具，支持分页。

```go
// internal/tools/read.go

type ReadTool struct {
    env      Environment
    maxLines int // 最大读取行数
}

func NewReadTool(env Environment) *ReadTool {
    return &ReadTool{
        env:      env,
        maxLines: 10000,
    }
}

func (t *ReadTool) Name() string {
    return "read"
}

func (t *ReadTool) Description() string {
    return "读取文件内容"
}

func (t *ReadTool) Parameters() json.RawMessage {
    return json.RawMessage(`{
        "type": "object",
        "properties": {
            "file_path": {
                "type": "string",
                "description": "要读取的文件路径"
            },
            "offset": {
                "type": "integer",
                "description": "起始行号（从 1 开始），默认为 1"
            },
            "limit": {
                "type": "integer",
                "description": "读取行数，默认读取全部"
            }
        },
        "required": ["file_path"]
    }`)
}

func (t *ReadTool) Execute(ctx context.Context, input map[string]any) (string, error) {
    filePath := input["file_path"].(string)
    offset := 0
    limit := -1

    if o, ok := input["offset"].(float64); ok {
        offset = int(o) - 1 // 转换为 0-based
    }
    if l, ok := input["limit"].(float64); ok {
        limit = int(l)
    }

    // 使用环境读取文件
    content, err := t.env.ReadFile(filePath)
    if err != nil {
        return "", fmt.Errorf("read failed: %w", err)
    }

    // 按行分割并应用偏移和限制
    lines := strings.Split(string(content), "\n")
    if offset > 0 {
        if offset >= len(lines) {
            return "", fmt.Errorf("offset exceeds file length")
        }
        lines = lines[offset:]
    }

    if limit > 0 && limit < len(lines) {
        lines = lines[:limit]
    }

    // 添加行号
    var result []string
    startLine := offset + 1
    for i, line := range lines {
        result = append(result, fmt.Sprintf("%5d→%s", startLine+i, line))
    }

    return strings.Join(result, "\n"), nil
}
```

---

## 5. P1 工具实现

### 5.1 Bash 工具

命令执行工具，支持异步执行。

```go
// internal/tools/bash.go

type BashTool struct {
    env             Environment
    timeout         time.Duration
    allowedCommands map[string]bool // 白名单（可选）
}

func NewBashTool(env Environment) *BashTool {
    return &BashTool{
        env:     env,
        timeout: 120 * time.Second,
    }
}

func (t *BashTool) Name() string {
    return "bash"
}

func (t *BashTool) Description() string {
    return "执行 shell 命令并返回输出"
}

func (t *BashTool) Parameters() json.RawMessage {
    return json.RawMessage(`{
        "type": "object",
        "properties": {
            "command": {
                "type": "string",
                "description": "要执行的命令"
            },
            "cwd": {
                "type": "string",
                "description": "工作目录"
            },
            "timeout": {
                "type": "integer",
                "description": "超时时间（秒），默认 120"
            }
        },
        "required": ["command"]
    }`)
}

func (t *BashTool) Execute(ctx context.Context, input map[string]any) (string, error) {
    command := input["command"].(string)
    cwd := ""
    timeout := t.timeout

    if c, ok := input["cwd"].(string); ok {
        cwd = c
    }
    if to, ok := input["timeout"].(float64); ok {
        timeout = time.Duration(to) * time.Second
    }

    // 创建带超时的 context
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    // 使用环境执行命令
    var output []byte
    var err error
    if cwd != "" {
        output, err = t.env.ExecCommandWithWorkingDir(ctx, cwd, "sh", "-c", command)
    } else {
        output, err = t.env.ExecCommand(ctx, "sh", "-c", command)
    }

    if err != nil {
        return "", fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
    }

    return string(output), nil
}
```

### 5.2 Write 工具

文件写入工具。

```go
// internal/tools/write.go

type WriteTool struct {
    env            Environment
    protectedPaths []string // 受保护的路径
}

func NewWriteTool(env Environment) *WriteTool {
    return &WriteTool{
        env:            env,
        protectedPaths: []string{".env", "credentials", "/etc"},
    }
}

func (t *WriteTool) Name() string {
    return "write"
}

func (t *WriteTool) Description() string {
    return "写入内容到文件（如果文件已存在将被覆盖）"
}

func (t *WriteTool) Parameters() json.RawMessage {
    return json.RawMessage(`{
        "type": "object",
        "properties": {
            "file_path": {
                "type": "string",
                "description": "文件路径"
            },
            "content": {
                "type": "string",
                "description": "要写入的内容"
            }
        },
        "required": ["file_path", "content"]
    }`)
}

func (t *WriteTool) Execute(ctx context.Context, input map[string]any) (string, error) {
    filePath := input["file_path"].(string)
    content := input["content"].(string)

    // 解析完整路径用于安全检查
    fullPath := t.env.ResolvePath(filePath)

    // 检查是否是受保护路径
    for _, protected := range t.protectedPaths {
        if strings.HasPrefix(fullPath, protected) || strings.HasPrefix(filePath, protected) {
            return "", fmt.Errorf("cannot write to protected path: %s", protected)
        }
    }

    // 使用环境写入文件
    if err := t.env.WriteFile(filePath, []byte(content), 0644); err != nil {
        return "", fmt.Errorf("write failed: %w", err)
    }

    return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), filePath), nil
}
```

### 5.3 Edit 工具

文件编辑工具，支持字符串替换。

```go
// internal/tools/edit.go

type EditTool struct {
    env Environment
}

func NewEditTool(env Environment) *EditTool {
    return &EditTool{env: env}
}

func (t *EditTool) Name() string {
    return "edit"
}

func (t *EditTool) Description() string {
    return "在文件中替换字符串"
}

func (t *EditTool) Parameters() json.RawMessage {
    return json.RawMessage(`{
        "type": "object",
        "properties": {
            "file_path": {
                "type": "string",
                "description": "文件路径"
            },
            "old_string": {
                "type": "string",
                "description": "要替换的字符串"
            },
            "new_string": {
                "type": "string",
                "description": "替换后的字符串"
            },
            "replace_all": {
                "type": "boolean",
                "description": "是否替换所有匹配项，默认只替换第一个"
            }
        },
        "required": ["file_path", "old_string", "new_string"]
    }`)
}

func (t *EditTool) Execute(ctx context.Context, input map[string]any) (string, error) {
    filePath := input["file_path"].(string)
    oldStr := input["old_string"].(string)
    newStr := input["new_string"].(string)
    replaceAll := false

    if ra, ok := input["replace_all"].(bool); ok {
        replaceAll = ra
    }

    // 使用环境读取文件
    content, err := t.env.ReadFile(filePath)
    if err != nil {
        return "", fmt.Errorf("read failed: %w", err)
    }

    // 执行替换
    var newContent string
    if replaceAll {
        newContent = strings.ReplaceAll(string(content), oldStr, newStr)
    } else {
        if !strings.Contains(string(content), oldStr) {
            return "", fmt.Errorf("old_string not found in file")
        }
        newContent = strings.Replace(string(content), oldStr, newStr, 1)
    }

    // 使用环境写回文件
    if err := t.env.WriteFile(filePath, []byte(newContent), 0644); err != nil {
        return "", fmt.Errorf("write failed: %w", err)
    }

    return fmt.Sprintf("Successfully replaced occurrence(s) in %s", filePath), nil
}
```

---

## 6. ReAct 循环集成

### 5.1 工具调用消息流

```
┌──────────────────────────────────────────────────────────┐
│                    LLM Response                           │
│  Content: "I'll search for..."                           │
│  ToolCalls:                                              │
│    - Name: "grep"                                        │
│      Input: {"pattern": "error", "file_pattern": "*.go"} │
└────────────────┬─────────────────────────────────────────┘
                 │
                 ▼
┌──────────────────────────────────────────────────────────┐
│               ToolCallMsg (TUI → Agent)                  │
│  MessageID: "msg-123"                                    │
│  ToolName: "grep"                                        │
│  Input: {...}                                            │
└────────────────┬─────────────────────────────────────────┘
                 │
                 ▼
┌──────────────────────────────────────────────────────────┐
│              ToolExecutor.Execute()                      │
│  1. 验证参数                                              │
│  2. 权限检查（Phase 4）                                    │
│  3. 执行工具                                              │
│  4. 返回结果                                              │
└────────────────┬─────────────────────────────────────────┘
                 │
                 ▼
┌──────────────────────────────────────────────────────────┐
│              ToolResultMsg (Agent → TUI)                 │
│  MessageID: "msg-123"                                    │
│  ToolName: "grep"                                        │
│  Output: "main.go:42: return err"                        │
│  Status: "completed"                                     │
└────────────────┬─────────────────────────────────────────┘
                 │
                 ▼
┌──────────────────────────────────────────────────────────┐
│            添加到消息 ReActLoop                           │
│  ReActStep{                                              │
│    Type: "observation",                                  │
│    ToolCall: {                                           │
│      Name: "grep",                                       │
│      Input: {...},                                       │
│      Output: "main.go:42: return err",                   │
│      Status: "completed"                                 │
│    }                                                     │
│  }                                                       │
└──────────────────────────────────────────────────────────┘
```

### 5.2 Agent 工具调用方法

```go
// internal/agent/agent.go

// ExecuteTool 执行工具调用
func (a *Agent) ExecuteTool(ctx context.Context, msgID, toolName string, input map[string]any) (string, error) {
    // 获取工具
    tool := a.toolRegistry.Get(toolName)
    if tool == nil {
        return "", fmt.Errorf("tool not found: %s", toolName)
    }

    // 参数验证
    if err := tool.Validate(input); err != nil {
        return "", fmt.Errorf("parameter validation failed: %w", err)
    }

    // 执行工具
    output, err := tool.Execute(ctx, input)
    if err != nil {
        return "", fmt.Errorf("tool execution failed: %w", err)
    }

    return output, nil
}
```

### 5.3 TUI 消息处理

```go
// internal/ui/handlers.go

// ToolCallMsg 工具调用消息
type ToolCallMsg struct {
    MessageID string
    ToolName  string
    Input     map[string]any
}

// ToolResultMsg 工具结果消息
type ToolResultMsg struct {
    MessageID string
    ToolName  string
    Output    string
    Status    message.Status
    Error     string
}

// handleToolCall 处理工具调用
func (m *Model) handleToolCall(msg ToolCallMsg) (tea.Model, tea.Cmd) {
    // 添加 action 步骤
    m.currentMessage.AddToolCall(msg.ToolName, msg.Input)

    // 异步执行工具
    return m, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
        output, err := m.agent.ExecuteTool(m.ctx, msg.MessageID, msg.ToolName, msg.Input)

        status := message.StatusCompleted
        errMsg := ""
        if err != nil {
            status = message.StatusError
            errMsg = err.Error()
            output = errMsg
        }

        return ToolResultMsg{
            MessageID: msg.MessageID,
            ToolName:  msg.ToolName,
            Output:    output,
            Status:    status,
            Error:     errMsg,
        }
    })
}

// handleToolResult 处理工具结果
func (m *Model) handleToolResult(msg ToolResultMsg) (tea.Model, tea.Cmd) {
    // 更新工具调用结果
    m.currentMessage.UpdateToolCall(msg.ToolName, msg.Output, msg.Status, msg.Error)

    // 继续下一轮 ReAct 循环
    return m, func() tea.Msg {
        // 调用 LLM 继续生成
        return m.agent.ContinueReAct(m.ctx, m.currentMessage)
    }
}
```

---

## 7. 工具注册与发现

### 6.1 工具注册表

```go
// internal/tools/registry.go

type Registry struct {
    tools map[string]Tool
}

func NewRegistry() *Registry {
    return &Registry{
        tools: make(map[string]Tool),
    }
}

func (r *Registry) Register(tool Tool) {
    r.tools[tool.Name()] = tool
}

func (r *Registry) Get(name string) Tool {
    return r.tools[name]
}

func (r *Registry) List() []Tool {
    result := make([]Tool, 0, len(r.tools))
    for _, tool := range r.tools {
        result = append(result, tool)
    }
    return result
}

// GetToolSchemas 获取所有工具的 schema（用于传递给 LLM）
func (r *Registry) GetToolSchemas() []map[string]any {
    schemas := make([]map[string]any, 0, len(r.tools))

    for _, tool := range r.tools {
        schema := map[string]any{
            "name":        tool.Name(),
            "description": tool.Description(),
            "input_schema": tool.Parameters(),
        }
        schemas = append(schemas, schema)
    }

    return schemas
}
```

### 6.2 初始化工具集

```go
// internal/agent/agent.go

func NewAgent(cfg *config.Config) (*Agent, error) {
    // ... 现有代码 ...

    // 创建执行环境（MVP 版本使用本地环境）
    env, err := tools.NewLocalEnvironment(cfg.WorkDir)
    if err != nil {
        return nil, fmt.Errorf("failed to create environment: %w", err)
    }

    // 创建工具注册表
    registry := tools.NewRegistry()

    // 注册 P0 工具（注入环境）
    registry.Register(tools.NewGlobTool(env))
    registry.Register(tools.NewGrepTool(env))
    registry.Register(tools.NewReadTool(env))

    // 注册 P1 工具（注入环境）
    bashTool := tools.NewBashTool(env)
    bashTool.Timeout = 120 * time.Second
    registry.Register(bashTool)

    registry.Register(tools.NewWriteTool(env))
    registry.Register(tools.NewEditTool(env))

    return &Agent{
        agent:    agent,
        config:   cfg,
        history:  history,
        tools:    registry,
        env:      env, // 保存环境引用用于清理
    }, nil
}
```

---

## 8. 验收测试场景

### 场景 1: 文件搜索任务

```
用户: "找出所有 .go 文件中包含 'error' 的行"

预期行为:
1. Agent: Thought → "我需要搜索 Go 文件中的 error 关键词"
2. Agent: Action → Call GlobTool(pattern="*.go")
3. Agent: Observation → 返回 ["main.go", "agent.go", "ui.go"]
4. Agent: Thought → "现在在这些文件中搜索 'error'"
5. Agent: Action → Call GrepTool(pattern="error", file_pattern="*.go")
6. Agent: Observation → 返回匹配行
7. Agent: Thought → "找到了所有包含 error 的行，现在总结结果"
8. Agent: 返回结果给用户
```

### 场景 2: 文件读取与编辑

```
用户: "读取 main.go 并把所有的 fmt.Printf 改成 log.Info"

预期行为:
1. Agent: ReadTool(file_path="main.go")
2. Agent: EditTool(file_path="main.go", old_string="fmt.Printf", new_string="log.Info", replace_all=true)
3. Agent: 确认修改成功
```

### 场景 3: 命令执行

```
用户: "运行 go test 并告诉我结果"

预期行为:
1. Agent: BashTool(command="go test")
2. Agent: 返回测试结果
```

---

## 9. 实现优先级

### Phase 3.1: 核心框架
- [ ] 执行环境接口 (`internal/tools/env.go`)
- [ ] 本地文件系统环境 (`internal/tools/env_local.go`)
- [ ] 工具接口定义 (`internal/tools/tool.go`)
- [ ] 工具注册表 (`internal/tools/registry.go`)
- [ ] 参数验证器 (`internal/tools/validator.go`)

### Phase 3.2: P0 工具
- [ ] Glob 工具 (`internal/tools/glob.go`)
- [ ] Grep 工具 (`internal/tools/grep.go`)
- [ ] Read 工具 (`internal/tools/read.go`)

### Phase 3.3: ReAct 循环
- [ ] Agent 工具调用方法 (`internal/agent/agent.go`)
- [ ] TUI 工具消息处理 (`internal/ui/handlers.go`)
- [ ] ReAct 消息流集成

### Phase 3.4: P1 工具
- [ ] Bash 工具 (`internal/tools/bash.go`)
- [ ] Write 工具 (`internal/tools/write.go`)
- [ ] Edit 工具 (`internal/tools/edit.go`)

### Phase 3.5: 测试与优化
- [ ] 单元测试
- [ ] 集成测试
- [ ] 性能优化
- [ ] 错误处理完善

---

## 10. 技术债务与未来改进

- **容器环境实现**: 实现 `ContainerEnvironment` 支持在 Docker 容器中执行工具
- **环境资源管理**: 添加环境资源限制（内存、CPU、磁盘）
- **并发工具执行**: 支持同时执行多个独立工具
- **工具链**: 工具输出可以作为另一个工具的输入
- **流式工具输出**: 长时间运行的工具可以流式返回输出
- **工具组合**: 创建复合工具（如 GitCommit = Write + Bash）
- **工具缓存**: 缓存工具结果以提高性能
- **智能工具选择**: 根据任务自动选择最佳工具组合
- **环境快照**: 支持创建环境的快照和回滚
