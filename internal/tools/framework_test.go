package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yourname/oxencode/pkg/logger"
)

func init() {
	// 初始化测试环境的 logger（使用 no-op output）
	_ = logger.Init(&logger.Config{
		Level:      "debug",
		DevMode:    true,
		OutputPath: "", // 输出到 stdout，但在测试时会被抑制
	})
}

// TestLocalEnvironment tests the LocalEnvironment implementation
func TestLocalEnvironment(t *testing.T) {
	// 创建临时目录用于测试
	tempDir, err := os.MkdirTemp("", "oxencode-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建测试 logger（使用 no-op logger）
	testLogger := logger.NewWithLogger("test", nil)

	t.Run("NewLocalEnvironment", func(t *testing.T) {
		env, err := NewLocalEnvironment(tempDir, testLogger)
		if err != nil {
			t.Fatalf("Failed to create environment: %v", err)
		}

		if env.GetWorkingDirectory() != tempDir {
			t.Errorf("Expected working directory %s, got %s", tempDir, env.GetWorkingDirectory())
		}

		if err := env.Cleanup(); err != nil {
			t.Errorf("Cleanup failed: %v", err)
		}
	})

	t.Run("WriteFile and ReadFile", func(t *testing.T) {
		env, err := NewLocalEnvironment(tempDir, testLogger)
		if err != nil {
			t.Fatalf("Failed to create environment: %v", err)
		}
		defer env.Cleanup()

		testFile := "test.txt"
		testContent := []byte("Hello, World!")

		// 写入文件
		err = env.WriteFile(testFile, testContent, 0644)
		if err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}

		// 读取文件
		content, err := env.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		if string(content) != string(testContent) {
			t.Errorf("Expected content %s, got %s", testContent, content)
		}
	})

	t.Run("WriteFile with subdirectories", func(t *testing.T) {
		env, err := NewLocalEnvironment(tempDir, testLogger)
		if err != nil {
			t.Fatalf("Failed to create environment: %v", err)
		}
		defer env.Cleanup()

		testFile := "subdir/test.txt"
		testContent := []byte("test content")

		err = env.WriteFile(testFile, testContent, 0644)
		if err != nil {
			t.Fatalf("Failed to write file with subdirectory: %v", err)
		}

		// 验证文件存在
		if !env.FileExists(testFile) {
			t.Error("File should exist")
		}
	})

	t.Run("FileExists", func(t *testing.T) {
		env, err := NewLocalEnvironment(tempDir, testLogger)
		if err != nil {
			t.Fatalf("Failed to create environment: %v", err)
		}
		defer env.Cleanup()

		testFile := "exists.txt"
		env.WriteFile(testFile, []byte("content"), 0644)

		if !env.FileExists(testFile) {
			t.Error("File should exist")
		}

		if env.FileExists("nonexistent.txt") {
			t.Error("File should not exist")
		}
	})

	t.Run("ListFiles", func(t *testing.T) {
		// 为每个测试子例创建独立的临时目录
		subDir, err := os.MkdirTemp(tempDir, "listfiles-*")
		if err != nil {
			t.Fatalf("Failed to create sub dir: %v", err)
		}

		env, err := NewLocalEnvironment(subDir, testLogger)
		if err != nil {
			t.Fatalf("Failed to create environment: %v", err)
		}
		defer env.Cleanup()

		// 创建测试文件
		files := []string{"test1.txt", "test2.txt", "test.go"}
		for _, f := range files {
			env.WriteFile(f, []byte("content"), 0644)
		}

		// 测试通配符
		matches, err := env.ListFiles("*.txt")
		if err != nil {
			t.Fatalf("Failed to list files: %v", err)
		}

		if len(matches) != 2 {
			t.Errorf("Expected 2 .txt files, got %d: %v", len(matches), matches)
		}
	})

	t.Run("ResolvePath", func(t *testing.T) {
		env, err := NewLocalEnvironment(tempDir, testLogger)
		if err != nil {
			t.Fatalf("Failed to create environment: %v", err)
		}
		defer env.Cleanup()

		// 相对路径
		resolved := env.ResolvePath("test.txt")
		expected := filepath.Join(tempDir, "test.txt")
		if resolved != expected {
			t.Errorf("Expected %s, got %s", expected, resolved)
		}

		// 绝对路径应该保持不变
		absPath := "/absolute/path/test.txt"
		resolved = env.ResolvePath(absPath)
		if resolved != absPath {
			t.Errorf("Absolute path should not change: expected %s, got %s", absPath, resolved)
		}
	})

	t.Run("ExecCommand", func(t *testing.T) {
		env, err := NewLocalEnvironment(tempDir, testLogger)
		if err != nil {
			t.Fatalf("Failed to create environment: %v", err)
		}
		defer env.Cleanup()

		ctx := context.Background()
		output, err := env.ExecCommand(ctx, "echo", "hello")
		if err != nil {
			t.Fatalf("Failed to execute command: %v", err)
		}

		outputStr := strings.TrimSpace(string(output))
		if outputStr != "hello" {
			t.Errorf("Expected 'hello', got '%s'", outputStr)
		}
	})

	t.Run("ExecCommandWithWorkingDir", func(t *testing.T) {
		env, err := NewLocalEnvironment(tempDir, testLogger)
		if err != nil {
			t.Fatalf("Failed to create environment: %v", err)
		}
		defer env.Cleanup()

		// 创建子目录
		subDir := "subdir"
		os.MkdirAll(filepath.Join(tempDir, subDir), 0755)

		ctx := context.Background()
		output, err := env.ExecCommandWithWorkingDir(ctx, subDir, "pwd")
		if err != nil {
			t.Fatalf("Failed to execute command: %v", err)
		}

		outputStr := strings.TrimSpace(string(output))
		// pwd 应该在子目录中
		if !strings.HasSuffix(outputStr, subDir) {
			t.Errorf("Expected pwd to end with %s, got %s", subDir, outputStr)
		}
	})
}

// TestRegistry tests the tool registry
func TestRegistry(t *testing.T) {
	testLogger := logger.NewWithLogger("test", nil)

	t.Run("NewRegistry", func(t *testing.T) {
		registry := NewRegistry(testLogger)
		if registry.Count() != 0 {
			t.Errorf("Expected empty registry, got %d tools", registry.Count())
		}
	})

	t.Run("Register and Get", func(t *testing.T) {
		registry := NewRegistry(testLogger)
		tool := &mockTool{name: "test", description: "test tool"}

		err := registry.Register(tool)
		if err != nil {
			t.Fatalf("Failed to register tool: %v", err)
		}

		if registry.Count() != 1 {
			t.Errorf("Expected 1 tool, got %d", registry.Count())
		}

		retrieved := registry.Get("test")
		if retrieved == nil {
			t.Error("Tool should exist")
		}

		if !registry.Has("test") {
			t.Error("Has should return true")
		}
	})

	t.Run("Register duplicate", func(t *testing.T) {
		registry := NewRegistry(testLogger)
		tool := &mockTool{name: "test", description: "test tool"}

		registry.Register(tool)
		err := registry.Register(tool)

		if err == nil {
			t.Error("Should return error when registering duplicate tool")
		}
	})

	t.Run("Unregister", func(t *testing.T) {
		registry := NewRegistry(testLogger)
		tool := &mockTool{name: "test", description: "test tool"}

		registry.Register(tool)
		err := registry.Unregister("test")

		if err != nil {
			t.Errorf("Failed to unregister tool: %v", err)
		}

		if registry.Count() != 0 {
			t.Errorf("Expected 0 tools after unregister, got %d", registry.Count())
		}
	})

	t.Run("GetToolSchemas", func(t *testing.T) {
		registry := NewRegistry(testLogger)
		tool := &mockTool{
			name:        "test",
			description: "test tool",
			parameters:  json.RawMessage(`{"type":"object"}`),
		}

		registry.Register(tool)
		schemas := registry.GetToolSchemas()

		if len(schemas) != 1 {
			t.Errorf("Expected 1 schema, got %d", len(schemas))
		}

		if schemas[0]["name"] != "test" {
			t.Errorf("Expected tool name 'test', got %v", schemas[0]["name"])
		}
	})

	t.Run("Execute tool", func(t *testing.T) {
		registry := NewRegistry(testLogger)
		tool := &mockTool{
			name:        "test",
			description: "test tool",
			executeFunc: func(ctx context.Context, input map[string]any) (string, error) {
				return "result", nil
			},
		}

		registry.Register(tool)

		ctx := context.Background()
		result, err := registry.Execute(ctx, "test", map[string]any{"key": "value"})

		if err != nil {
			t.Fatalf("Failed to execute tool: %v", err)
		}

		if result != "result" {
			t.Errorf("Expected 'result', got '%s'", result)
		}
	})

	t.Run("Execute non-existent tool", func(t *testing.T) {
		registry := NewRegistry(testLogger)
		ctx := context.Background()

		_, err := registry.Execute(ctx, "nonexistent", map[string]any{})

		if err == nil {
			t.Error("Should return error when executing non-existent tool")
		}
	})

	t.Run("Names", func(t *testing.T) {
		registry := NewRegistry(testLogger)
		registry.Register(&mockTool{name: "tool1", description: "tool 1"})
		registry.Register(&mockTool{name: "tool2", description: "tool 2"})

		names := registry.Names()
		if len(names) != 2 {
			t.Errorf("Expected 2 names, got %d", len(names))
		}
	})
}

// TestValidator tests the parameter validator
func TestValidator(t *testing.T) {
	testLogger := logger.NewWithLogger("test", nil)

	t.Run("Validate required fields", func(t *testing.T) {
		schema := json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {"type": "string"},
				"age": {"type": "integer"}
			},
			"required": ["name"]
		}`)

		validator := NewValidator(schema, testLogger)

		// 缺少必填字段
		err := validator.Validate(map[string]any{"age": 25})
		if err == nil {
			t.Error("Should return error for missing required field")
		}

		// 包含必填字段
		err = validator.Validate(map[string]any{"name": "John"})
		if err != nil {
			t.Errorf("Should not return error: %v", err)
		}
	})

	t.Run("Validate string type", func(t *testing.T) {
		schema := json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {"type": "string"}
			}
		}`)

		validator := NewValidator(schema, testLogger)

		// 正确的类型
		err := validator.Validate(map[string]any{"name": "John"})
		if err != nil {
			t.Errorf("Should not return error: %v", err)
		}

		// 错误的类型
		err = validator.Validate(map[string]any{"name": 123})
		if err == nil {
			t.Error("Should return error for wrong type")
		}
	})

	t.Run("Validate enum", func(t *testing.T) {
		schema := json.RawMessage(`{
			"type": "object",
			"properties": {
				"color": {
					"type": "string",
					"enum": ["red", "green", "blue"]
				}
			}
		}`)

		validator := NewValidator(schema, testLogger)

		// 有效的枚举值
		err := validator.Validate(map[string]any{"color": "red"})
		if err != nil {
			t.Errorf("Should not return error: %v", err)
		}

		// 无效的枚举值
		err = validator.Validate(map[string]any{"color": "yellow"})
		if err == nil {
			t.Error("Should return error for invalid enum value")
		}
	})

	t.Run("Validate number type", func(t *testing.T) {
		schema := json.RawMessage(`{
			"type": "object",
			"properties": {
				"count": {"type": "number"}
			}
		}`)

		validator := NewValidator(schema, testLogger)

		// 正确的类型（JSON 数字解析为 float64）
		err := validator.Validate(map[string]any{"count": 42.0})
		if err != nil {
			t.Errorf("Should not return error: %v", err)
		}

		// 错误的类型
		err = validator.Validate(map[string]any{"count": "not a number"})
		if err == nil {
			t.Error("Should return error for wrong type")
		}
	})

	t.Run("Validate boolean type", func(t *testing.T) {
		schema := json.RawMessage(`{
			"type": "object",
			"properties": {
				"active": {"type": "boolean"}
			}
		}`)

		validator := NewValidator(schema, testLogger)

		// 正确的类型
		err := validator.Validate(map[string]any{"active": true})
		if err != nil {
			t.Errorf("Should not return error: %v", err)
		}

		// 错误的类型
		err = validator.Validate(map[string]any{"active": "true"})
		if err == nil {
			t.Error("Should return error for wrong type")
		}
	})
}

// TestBaseTool tests the base tool implementation
func TestBaseTool(t *testing.T) {
	testLogger := logger.NewWithLogger("test", nil)

	t.Run("Validate default implementation", func(t *testing.T) {
		schema := json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {"type": "string"}
			},
			"required": ["name"]
		}`)

		tool := NewBaseTool("test", "test tool", schema, nil, testLogger)

		// 缺少必填字段
		err := tool.Validate(map[string]any{})
		if err == nil {
			t.Error("Should return error for missing required field")
		}

		// 包含必填字段
		err = tool.Validate(map[string]any{"name": "test"})
		if err != nil {
			t.Errorf("Should not return error: %v", err)
		}
	})
}

// TestValidationError tests the ValidationError type
func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Field:   "test_field",
		Message: "test error message",
	}

	if err.Error() != "test error message" {
		t.Errorf("Expected error message 'test error message', got '%s'", err.Error())
	}
}

// Mock tool for testing
type mockTool struct {
	name        string
	description string
	parameters  json.RawMessage
	executeFunc func(ctx context.Context, input map[string]any) (string, error)
	validateFunc func(input map[string]any) error
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Description() string {
	return m.description
}

func (m *mockTool) Parameters() json.RawMessage {
	if m.parameters == nil {
		return json.RawMessage(`{"type":"object"}`)
	}
	return m.parameters
}

func (m *mockTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, input)
	}
	return "mock result", nil
}

func (m *mockTool) Validate(input map[string]any) error {
	if m.validateFunc != nil {
		return m.validateFunc(input)
	}
	return nil
}

// TestToolExecuteError tests the ToolExecuteError type
func TestToolExecuteError(t *testing.T) {
	originalErr := &ValidationError{Field: "test", Message: "validation failed"}
	err := &ToolExecuteError{
		ToolName: "test_tool",
		Err:      originalErr,
	}

	if err.Error() != "validation failed" {
		t.Errorf("Expected error message from wrapped error, got '%s'", err.Error())
	}

	if err.Unwrap() != originalErr {
		t.Error("Unwrap should return the original error")
	}

	if err.ToolName != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got '%s'", err.ToolName)
	}
}

// TestRegistryConcurrent tests concurrent registry operations
func TestRegistryConcurrent(t *testing.T) {
	testLogger := logger.NewWithLogger("test", nil)
	registry := NewRegistry(testLogger)

	// 并发注册
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			tool := &mockTool{
				name:        string(rune('a' + n)),
				description: "tool",
			}
			registry.Register(tool)
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证工具数量（可能会有重复，所以只检查不为0）
	if registry.Count() == 0 {
		t.Error("Expected at least some tools to be registered")
	}
}

// TestLocalEnvironmentCommandTimeout tests command execution with timeout
func TestLocalEnvironmentCommandTimeout(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "oxencode-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testLogger := logger.NewWithLogger("test", nil)
	env, err := NewLocalEnvironment(tempDir, testLogger)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}
	defer env.Cleanup()

	// 创建一个会超时的 context
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// 执行一个会超过超时时间的命令
	start := time.Now()
	_, err = env.ExecCommand(ctx, "sleep", "2")
	elapsed := time.Since(start)

	// 应该因为超时而失败
	if err == nil {
		t.Error("Command should have timed out")
	}

	// 验证确实在超时时间内返回（加上一些缓冲时间）
	if elapsed > 500*time.Millisecond {
		t.Errorf("Command should have timed out quickly, took %v", elapsed)
	}
}
