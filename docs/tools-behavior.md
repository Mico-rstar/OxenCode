# 工具系统行为文档

本文档详细描述 OxenCode 工具系统中每个工具的具体行为、参数、返回值和使用示例。

---

## 目录

1. [工具概述](#工具概述)
2. [P0 工具](#p0-工具)
   - [Glob - 文件路径模式匹配](#glob---文件路径模式匹配)
   - [Grep - 内容搜索](#grep---内容搜索)
   - [Read - 文件读取](#read---文件读取)
3. [工具执行流程](#工具执行流程)
4. [错误处理](#错误处理)
5. [最佳实践](#最佳实践)

---

## 工具概述

工具系统是 Agent 与文件系统交互的核心机制。每个工具都实现统一的接口，通过 `Environment` 抽象层与文件系统交互，确保了安全性和可扩展性。

### 工具特性

- **环境隔离**: 所有工具通过 `Environment` 接口操作，支持本地文件系统和未来容器环境
- **参数验证**: 使用 JSON Schema 进行严格的参数验证
- **结构化日志**: 所有操作都有详细的日志记录
- **资源限制**: 内置资源限制，防止意外消耗过多资源

---

## P0 工具

P0 工具是 Agent 探索代码库的最低需求，提供了文件查找、内容搜索和文件读取功能。

### Glob - 文件路径模式匹配

**文件**: [internal/tools/glob.go](../internal/tools/glob.go)

#### 功能描述

使用通配符模式查找文件路径，支持常见的通配符语法。

#### 参数

| 参数名 | 类型 | 必填 | 描述 |
|--------|------|------|------|
| `pattern` | string | ✅ | 文件匹配模式，支持 `*`, `**`, `?` 等通配符 |
| `path` | string | ❌ | 搜索路径，默认为当前目录 `.` |

#### 返回值

**成功时**: 返回匹配的文件路径列表，每行一个路径
```
internal/tools/env.go
internal/tools/tool.go
internal/tools/registry.go
```

**无结果时**: 返回 `No matches found`

**失败时**: 返回错误信息，包含失败原因

#### 通配符语法

| 通配符 | 描述 | 示例 |
|--------|------|------|
| `*` | 匹配任意数量的字符（不包括 `/`） | `*.go` 匹配所有 Go 文件 |
| `**` | 匹配任意数量的字符（包括 `/`） | `**/*.go` 递归匹配所有 Go 文件 |
| `?` | 匹配单个字符 | `file?.txt` 匹配 `file1.txt`, `filea.txt` |
| `[abc]` | 匹配括号内的任意一个字符 | `[fb]oo.go` 匹配 `foo.go` 和 `boo.go` |
| `[!abc]` | 匹配不在括号内的字符 | `[!fb]oo.go` 不匹配 `foo.go` 和 `boo.go` |

#### 使用示例

```go
// 查找所有 Go 文件
result, err := registry.Execute(ctx, "glob", map[string]any{
    "pattern": "*.go",
})

// 查找 internal 目录下的所有测试文件
result, err := registry.Execute(ctx, "glob", map[string]any{
    "pattern": "*_test.go",
    "path":    "internal",
})

// 递归查找所有 markdown 文件
result, err := registry.Execute(ctx, "glob", map[string]any{
    "pattern": "**/*.md",
})
```

#### 行为细节

1. **路径解析**: 所有路径都相对于工作目录解析
2. **相对路径**: 返回的路径都是相对于工作目录的相对路径
3. **大小写敏感**: 在 Unix 系统上区分大小写，Windows 上不区分
4. **性能**: 对于大型代码库，使用更具体的模式可以提高性能

#### 典型应用场景

- 查找特定类型的文件（如 `*.go`, `*.md`）
- 探索目录结构
- 定位测试文件
- 查找配置文件

---

### Grep - 内容搜索

**文件**: [internal/tools/grep.go](../internal/tools/grep.go)

#### 功能描述

在文件中搜索匹配正则表达式的内容，支持复杂的搜索模式。

#### 参数

| 参数名 | 类型 | 必填 | 描述 |
|--------|------|------|------|
| `pattern` | string | ✅ | 正则表达式搜索模式 |
| `path` | string | ❌ | 搜索路径，默认为当前目录 `.` |
| `file_pattern` | string | ❌ | 文件过滤模式（如 `*.go`） |
| `ignore_case` | boolean | ❌ | 是否忽略大小写，默认 `false` |

#### 返回值

**成功时**: 返回匹配的行，格式为 `文件路径:行号:内容`
```
internal/tools/grep.go:12:package tools
internal/tools/grep.go:17:// GrepTool 内容搜索工具
internal/tools/read.go:12:package tools
```

**无结果时**: 返回 `No matches found`

**结果过多时**: 返回前 1000 条结果，并提示 `(X more results truncated)`

**失败时**: 返回错误信息

#### 正则表达式语法

支持 Go 的正则表达式语法（RE2），常用模式：

| 模式 | 描述 | 示例 |
|------|------|------|
| `abc` | 字面匹配 | 匹配 "abc" |
| `^abc` | 行首匹配 | 匹配行首的 "abc" |
| `abc$` | 行尾匹配 | 匹配行尾的 "abc" |
| `a.b` | `.` 匹配任意字符 | "aab", "acb", "a1b" |
| `a.*b` | `.*` 匹配任意数量字符 | "ab", "axxxxxb" |
| `(a\|b)` | 或运算 | 匹配 "a" 或 "b" |
| `[abc]` | 字符集 | 匹配 "a", "b", 或 "c" |
| `\d` | 数字字符 | 匹配 0-9 |
| `\w` | 单词字符 | 匹配字母、数字、下划线 |

#### 使用示例

```go
// 搜索包含 "type Tool" 的行
result, err := registry.Execute(ctx, "grep", map[string]any{
    "pattern": "type Tool",
})

// 在 Go 文件中搜索 "func main"
result, err := registry.Execute(ctx, "grep", map[string]any{
    "pattern":      "func main",
    "file_pattern": "*.go",
})

// 忽略大小写搜索 "error"
result, err := registry.Execute(ctx, "grep", map[string]any{
    "pattern":      "error",
    "ignore_case":  true,
    "path":         "internal",
})

// 搜索函数定义（正则表达式）
result, err := registry.Execute(ctx, "grep", map[string]any{
    "pattern":      `^func \w+`,
    "file_pattern": "*.go",
})
```

#### 行为细节

1. **行号**: 行号从 1 开始计数
2. **编码**: 假设文件为 UTF-8 编码
3. **二进制文件**: 自动跳过二进制文件
4. **递归搜索**: 默认递归搜索所有子目录
5. **结果限制**: 最多返回 1000 条匹配结果
6. **并发**: 单线程顺序搜索，避免资源竞争

#### 典型应用场景

- 查找函数定义
- 搜索变量使用
- 定位 TODO/FIXME 注释
- 查找导入语句
- 搜索特定模式（如错误处理）

#### 性能考虑

- 使用 `file_pattern` 限制搜索范围可以显著提高性能
- 简单的正则表达式比复杂的快
- 搜索小文件比搜索大文件快

---

### Read - 文件读取

**文件**: [internal/tools/read.go](../internal/tools/read.go)

#### 功能描述

读取文件内容，支持分页读取（通过 offset 和 limit），适合查看大文件。

#### 参数

| 参数名 | 类型 | 必填 | 描述 |
|--------|------|------|------|
| `file_path` | string | ✅ | 要读取的文件路径 |
| `offset` | integer | ❌ | 起始行号（从 1 开始），默认为 1 |
| `limit` | integer | ❌ | 读取行数，默认读取全部 |

#### 返回值

**成功时**: 返回带行号的文件内容
```
    1→package tools
    2→
    3→import (
    4→    "context"
    5→    "encoding/json"
    6→    "fmt"
    7→)
```

**空文件**: 返回 `File is empty`

**失败时**: 返回错误信息，包含失败原因

#### 行号格式

- 行号占据 5 个字符宽度，右对齐
- 行号后跟 `→` 符号
- 格式: `%5d→%s`

#### 使用示例

```go
// 读取整个文件
result, err := registry.Execute(ctx, "read", map[string]any{
    "file_path": "internal/tools/tool.go",
})

// 读取文件的前 20 行
result, err := registry.Execute(ctx, "read", map[string]any{
    "file_path": "README.md",
    "limit":     20,
})

// 从第 100 行开始读取 50 行
result, err := registry.Execute(ctx, "read", map[string]any{
    "file_path": "large_file.log",
    "offset":    100,
    "limit":     50,
})
```

#### 行为细节

1. **行号**: 行号从 1 开始（用户可见），内部使用 0-based 索引
2. **空行**: 空行也会被计数和显示
3. **行尾**: 保留原始的行尾字符（`\n` 或 `\r\n`）
4. **最大行数**: 单次读取最多 10000 行
5. **空文件**: 返回明确的消息而非空字符串
6. **编码**: 假设文件为 UTF-8 编码

#### 分页示例

假设文件有 100 行：

```go
// 第1页（行 1-20）
offset: 1,  limit: 20

// 第2页（行 21-40）
offset: 21, limit: 20

// 第3页（行 41-60）
offset: 41, limit: 20

// 第4页（行 61-80）
offset: 61, limit: 20

// 第5页（行 81-100）
offset: 81, limit: 20
```

#### 典型应用场景

- 查看源代码
- 读取配置文件
- 查看日志文件（使用分页）
- 读取文档文件
- 检查测试文件

#### 错误处理

| 错误场景 | 返回消息 |
|---------|---------|
| 文件不存在 | `read failed: open ...: no such file or directory` |
| 无权限访问 | `read failed: open ...: permission denied` |
| offset 超出范围 | `offset X exceeds file length (Y lines)` |
| 文件为空 | `File is empty` |

---

## 工具执行流程

所有工具都遵循统一的执行流程：

```
1. 接收工具调用请求
   ↓
2. 参数验证（JSON Schema）
   ↓
3. 环境检查（文件存在性、权限等）
   ↓
4. 执行工具逻辑
   ↓
5. 格式化输出
   ↓
6. 返回结果
```

### 执行上下文

每个工具都接收 `context.Context` 参数，支持：
- 超时控制
- 取消操作
- 上下文传递

### 环境隔离

所有工具通过 `Environment` 接口与文件系统交互：

```go
type Environment interface {
    GetWorkingDirectory() string
    ReadFile(path string) ([]byte, error)
    WriteFile(path string, data []byte, perm fs.FileMode) error
    ListFiles(pattern string) ([]string, error)
    FileExists(path string) bool
    ExecCommand(ctx context.Context, cmd string, args ...string) ([]byte, error)
    // ...
}
```

当前实现：`LocalEnvironment` - 本地文件系统
未来实现：`ContainerEnvironment` - 容器隔离环境

---

## 错误处理

### 错误分类

1. **参数错误**: 缺少必填参数、参数类型错误
2. **验证错误**: 正则表达式无效、路径格式错误
3. **执行错误**: 文件不存在、权限不足
4. **系统错误**: 磁盘满、IO 错误

### 错误返回格式

所有错误都包含描述性消息：

```go
// 参数错误
"pattern must be a string"

// 执行错误
"glob failed: no matching files found"

// 验证错误
"invalid regex: error parsing regexp: missing closing ]"
```

### 日志记录

所有工具操作都记录结构化日志：

```json
{
  "level": "debug",
  "msg": "Executing glob",
  "pattern": "*.go",
  "path": "."
}

{
  "level": "info",
  "msg": "Glob completed",
  "matchCount": 42
}
```

---

## 最佳实践

### 1. 组合使用工具

**场景**: 查找并查看特定类型的文件

```go
// 1. 使用 Glob 查找所有 Go 文件
files, _ := registry.Execute(ctx, "glob", map[string]any{
    "pattern": "**/*.go",
})

// 2. 使用 Grep 搜索包含 "TODO" 的文件
results, _ := registry.Execute(ctx, "grep", map[string]any{
    "pattern":      "TODO",
    "file_pattern": "*.go",
})

// 3. 使用 Read 查看具体文件
content, _ := registry.Execute(ctx, "read", map[string]any{
    "file_path": "internal/tools/tool.go",
})
```

### 2. 优化搜索性能

```go
// ❌ 低效：搜索所有文件
result, _ := registry.Execute(ctx, "grep", map[string]any{
    "pattern": "func main",
})

// ✅ 高效：限制在 Go 文件中搜索
result, _ := registry.Execute(ctx, "grep", map[string]any{
    "pattern":      "func main",
    "file_pattern": "*.go",
})
```

### 3. 分页读取大文件

```go
const pageSize = 100

for page := 0; ; page++ {
    offset := page*pageSize + 1
    result, err := registry.Execute(ctx, "read", map[string]any{
        "file_path": "large.log",
        "offset":    offset,
        "limit":     pageSize,
    })

    if err != nil || strings.Contains(result, "offset exceeds") {
        break
    }

    // 处理这一页的内容...
}
```

### 4. 正则表达式技巧

```go
// 精确匹配单词
`bword\b`

// 匹配函数定义
`^func\s+\w+`

// 匹配导入语句
`^import\s+`

// 匹配结构体定义
`type\s+\w+\s+struct\s*{`
```

### 5. 错误处理

```go
result, err := registry.Execute(ctx, "read", map[string]any{
    "file_path": "config.toml",
})

if err != nil {
    if strings.Contains(err.Error(), "no such file") {
        // 文件不存在，使用默认配置
    } else if strings.Contains(err.Error(), "permission denied") {
        // 权限错误
    }
}
```

---

## 性能指标

### 典型操作性能

| 操作 | 文件数/大小 | 耗时 |
|------|------------|------|
| Glob `*.go` | 500 个文件 | ~10ms |
| Grep 搜索（小文件） | 100 个文件 | ~50ms |
| Grep 搜索（大文件） | 1000 个文件 | ~500ms |
| Read（1KB 文件） | 1KB | ~1ms |
| Read（1MB 文件） | 1MB | ~10ms |

### 优化建议

1. **使用 file_pattern**: 限制搜索范围可以减少 80-90% 的搜索时间
2. **避免递归 Glob**: 使用更具体的路径而非 `**`
3. **分页读取**: 对于大文件，使用 offset/limit 避免一次性读取
4. **缓存结果**: 多次使用的文件可以缓存内容

---

## 安全考虑

### 文件访问控制

- 所有文件操作都在工作目录内
- 路径遍历攻击被阻止
- 符号链接被正确处理

### 资源限制

- Grep: 最多返回 1000 条结果
- Read: 最多读取 10000 行
- 超时控制通过 context 实现

### 输入验证

- 所有参数都经过 JSON Schema 验证
- 正则表达式在编译时验证
- 路径在操作前验证

---

## 扩展性

### 添加新工具

1. 创建新文件 `internal/tools/newtool.go`
2. 实现 `Tool` 接口
3. 添加 JSON Schema 参数定义
4. 实现业务逻辑
5. 编写单元测试

### 工具接口

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() json.RawMessage
    Execute(ctx context.Context, input map[string]any) (string, error)
    Validate(input map[string]any) error
}
```

---

## 相关文档

- [工具系统设计文档](wip/tool.md) - 架构设计和实现细节
- [ReAct Loop 文档](wip/react.md) - Agent 如何使用工具
- [API 文档](https://pkg.go.dev/github.com/yourname/oxencode/internal/tools) - Go API 参考

---

**最后更新**: 2026-02-17
**版本**: v0.1.0
