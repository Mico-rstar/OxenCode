package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yourname/oxencode/pkg/logger"
)

func init() {
	// 初始化测试环境的 logger
	_ = logger.Init(&logger.Config{
		Level:      "debug",
		DevMode:    true,
		OutputPath: "",
	})
}

// TestBashTool_BasicExecution 测试 Bash 工具基本执行
func TestBashTool_BasicExecution(t *testing.T) {
	_, env := setupTestEnv(t)
	tool := NewBashTool(env, nil)

	ctx := context.Background()

	// 测试 echo 命令
	result, err := tool.Execute(ctx, map[string]any{
		"command": "echo",
		"args":    []string{"hello", "world"},
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// 验证输出包含 hello world
	if !strings.Contains(result, "hello") || !strings.Contains(result, "world") {
		t.Errorf("Expected output to contain 'hello world', got: %s", result)
	}
}

// TestBashTool_WithTimeout 测试 Bash 工具超时
func TestBashTool_WithTimeout(t *testing.T) {
	_, env := setupTestEnv(t)
	tool := NewBashTool(env, nil)

	ctx := context.Background()

	// 测试超时设置
	result, err := tool.Execute(ctx, map[string]any{
		"command": "echo",
		"args":    []string{"timeout test"},
		"timeout": float64(10),
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !strings.Contains(result, "timeout test") {
		t.Errorf("Expected output to contain 'timeout test', got: %s", result)
	}
}

// TestBashTool_CommandNotFound 测试 Bash 工具命令不存在
func TestBashTool_CommandNotFound(t *testing.T) {
	_, env := setupTestEnv(t)
	tool := NewBashTool(env, nil)

	ctx := context.Background()

	// 执行一个不存在的命令
	result, err := tool.Execute(ctx, map[string]any{
		"command": "nonexistentcommand12345",
	})

	// Bash 工具不会返回错误，而是返回包含错误信息的输出
	if err != nil {
		t.Fatalf("Execute should not fail for command not found: %v", err)
	}

	if !strings.Contains(result, "failed") && !strings.Contains(result, "not found") {
		t.Logf("Expected error message in output, got: %s", result)
	}
}

// TestWriteTool_CreateNewFile 测试 Write 工具创建新文件
func TestWriteTool_CreateNewFile(t *testing.T) {
	_, env := setupTestEnv(t)
	tool := NewWriteTool(env, nil)

	ctx := context.Background()

	testFile := "test_write.txt"
	content := "Hello, World!"

	result, err := tool.Execute(ctx, map[string]any{
		"file_path": testFile,
		"content":   content,
		"mode":      "create",
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !strings.Contains(result, "created") {
		t.Errorf("Expected 'created' in result, got: %s", result)
	}

	// 验证文件存在
	fullPath := filepath.Join(env.GetWorkingDirectory(), testFile)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		t.Errorf("File was not created: %s", fullPath)
	}

	// 验证内容
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(data) != content {
		t.Errorf("Expected content %q, got %q", content, string(data))
	}
}

// TestWriteTool_OverwriteFile 测试 Write 工具覆盖文件
func TestWriteTool_OverwriteFile(t *testing.T) {
	_, env := setupTestEnv(t)
	tool := NewWriteTool(env, nil)

	ctx := context.Background()

	testFile := "test_overwrite.txt"
	initialContent := "Initial content"

	// 创建文件
	_, err := tool.Execute(ctx, map[string]any{
		"file_path": testFile,
		"content":   initialContent,
		"mode":      "create",
	})

	if err != nil {
		t.Fatalf("Initial create failed: %v", err)
	}

	// 覆盖文件
	newContent := "New content"
	result, err := tool.Execute(ctx, map[string]any{
		"file_path": testFile,
		"content":   newContent,
		"mode":      "overwrite",
	})

	if err != nil {
		t.Fatalf("Overwrite failed: %v", err)
	}

	if !strings.Contains(result, "overwritten") && !strings.Contains(result, "successfully") {
		t.Logf("Result: %s", result)
	}

	// 验证内容已更新
	fullPath := filepath.Join(env.GetWorkingDirectory(), testFile)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(data) != newContent {
		t.Errorf("Expected content %q, got %q", newContent, string(data))
	}
}

// TestWriteTool_CreateModeFailsOnExisting 测试 Write 工具 create 模式在文件存在时失败
func TestWriteTool_CreateModeFailsOnExisting(t *testing.T) {
	_, env := setupTestEnv(t)
	tool := NewWriteTool(env, nil)

	ctx := context.Background()

	testFile := "test_create_fail.txt"

	// 创建文件
	_, err := tool.Execute(ctx, map[string]any{
		"file_path": testFile,
		"content":   "initial",
		"mode":      "create",
	})

	if err != nil {
		t.Fatalf("Initial create failed: %v", err)
	}

	// 尝试再次创建（应该失败）
	_, err = tool.Execute(ctx, map[string]any{
		"file_path": testFile,
		"content":   "new content",
		"mode":      "create",
	})

	if err == nil {
		t.Error("Expected error when creating existing file with create mode")
	}

	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Expected 'already exists' error, got: %v", err)
	}
}

// TestEditTool_SingleReplacement 测试 Edit 工具单次替换
func TestEditTool_SingleReplacement(t *testing.T) {
	_, env := setupTestEnv(t)
	tool := NewEditTool(env, nil)

	ctx := context.Background()

	// 创建测试文件
	testFile := "test_edit.txt"
	initialContent := "Hello World\nHello World\nHello World"
	fullPath := filepath.Join(env.GetWorkingDirectory(), testFile)

	if err := os.WriteFile(fullPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// 执行替换（仅替换第一个）
	result, err := tool.Execute(ctx, map[string]any{
		"file_path":   testFile,
		"old_string":  "Hello",
		"new_string":  "Hi",
		"replace_all": false,
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !strings.Contains(result, "1 replacement") {
		t.Errorf("Expected '1 replacement' in result, got: %s", result)
	}

	// 验证内容
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	content := string(data)
	expected := "Hi World\nHello World\nHello World"
	if content != expected {
		t.Errorf("Expected content %q, got %q", expected, content)
	}
}

// TestEditTool_ReplaceAll 测试 Edit 工具替换所有
func TestEditTool_ReplaceAll(t *testing.T) {
	_, env := setupTestEnv(t)
	tool := NewEditTool(env, nil)

	ctx := context.Background()

	// 创建测试文件
	testFile := "test_edit_all.txt"
	initialContent := "Hello World\nHello World\nHello World"
	fullPath := filepath.Join(env.GetWorkingDirectory(), testFile)

	if err := os.WriteFile(fullPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// 执行替换所有
	result, err := tool.Execute(ctx, map[string]any{
		"file_path":   testFile,
		"old_string":  "Hello",
		"new_string":  "Hi",
		"replace_all": true,
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !strings.Contains(result, "3 replacement") {
		t.Errorf("Expected '3 replacement' in result, got: %s", result)
	}

	// 验证内容
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	content := string(data)
	expected := "Hi World\nHi World\nHi World"
	if content != expected {
		t.Errorf("Expected content %q, got %q", expected, content)
	}
}

// TestEditTool_BatchReplacements 测试 Edit 工具批量替换
func TestEditTool_BatchReplacements(t *testing.T) {
	_, env := setupTestEnv(t)
	tool := NewEditTool(env, nil)

	ctx := context.Background()

	// 创建测试文件
	testFile := "test_edit_batch.txt"
	initialContent := "The quick brown fox jumps over the lazy dog"
	fullPath := filepath.Join(env.GetWorkingDirectory(), testFile)

	if err := os.WriteFile(fullPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// 执行批量替换
	result, err := tool.Execute(ctx, map[string]any{
		"file_path": testFile,
		"replacements": []any{
			map[string]any{"old_string": "quick", "new_string": "slow"},
			map[string]any{"old_string": "brown", "new_string": "red"},
			map[string]any{"old_string": "lazy", "new_string": "active"},
		},
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !strings.Contains(result, "3 replacement") {
		t.Errorf("Expected '3 replacement' in result, got: %s", result)
	}

	// 验证内容
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	content := string(data)
	expected := "The slow red fox jumps over the active dog"
	if content != expected {
		t.Errorf("Expected content %q, got %q", expected, content)
	}
}

// TestEditTool_NoMatch 测试 Edit 工具无匹配
func TestEditTool_NoMatch(t *testing.T) {
	_, env := setupTestEnv(t)
	tool := NewEditTool(env, nil)

	ctx := context.Background()

	// 创建测试文件
	testFile := "test_edit_nomatch.txt"
	initialContent := "Hello World"

	if err := os.WriteFile(filepath.Join(env.GetWorkingDirectory(), testFile), []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// 执行不存在的替换
	result, err := tool.Execute(ctx, map[string]any{
		"file_path":  testFile,
		"old_string": "Goodbye",
		"new_string": "Hello",
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !strings.Contains(result, "No changes made") {
		t.Errorf("Expected 'No changes made' in result, got: %s", result)
	}
}

// TestEditTool_CreateBackup 测试 Edit 工具创建备份
func TestEditTool_CreateBackup(t *testing.T) {
	_, env := setupTestEnv(t)
	tool := NewEditTool(env, nil)

	ctx := context.Background()

	// 创建测试文件
	testFile := "test_edit_backup.txt"
	initialContent := "Original content"
	fullPath := filepath.Join(env.GetWorkingDirectory(), testFile)

	if err := os.WriteFile(fullPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// 执行替换并创建备份
	_, err := tool.Execute(ctx, map[string]any{
		"file_path":     testFile,
		"old_string":    "Original",
		"new_string":    "Modified",
		"create_backup": true,
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// 验证备份文件存在
	backupPath := fullPath + ".bak"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Errorf("Backup file was not created: %s", backupPath)
	}

	// 验证备份内容
	backupData, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("Failed to read backup file: %v", err)
	}

	if string(backupData) != initialContent {
		t.Errorf("Backup content mismatch: expected %q, got %q", initialContent, string(backupData))
	}
}

// TestBashTool_BackgroundExecution 测试后台执行
func TestBashTool_BackgroundExecution(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping background execution test in CI")
	}

	_, env := setupTestEnv(t)
	tool := NewBashTool(env, nil)

	ctx := context.Background()

	// 在 Windows 上使用 timeout 命令，在 Unix 上使用 sleep
	command := "sleep"
	args := []string{"1"}
	if os.Getenv("OS") == "Windows_NT" {
		command = "timeout"
		args = []string{"/t", "1"}
	}

	result, err := tool.Execute(ctx, map[string]any{
		"command":    command,
		"args":       args,
		"background": true,
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !strings.Contains(result, "Background command started") && !strings.Contains(result, "PID") {
		t.Logf("Result: %s", result)
	}
}

// BenchmarkBashTool_SimpleCommand 性能基准测试
func BenchmarkBashTool_SimpleCommand(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "oxencode-bench-bash-*")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { os.RemoveAll(tempDir) })

	log := logger.New("test")
	env, err := NewLocalEnvironment(tempDir, log)
	if err != nil {
		b.Fatal(err)
	}

	tool := NewBashTool(env, nil)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tool.Execute(ctx, map[string]any{
			"command": "echo",
			"args":    []string{"test"},
		})
	}
}

// BenchmarkWriteTool_SmallFile 写入小文件性能基准测试
func BenchmarkWriteTool_SmallFile(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "oxencode-bench-write-*")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { os.RemoveAll(tempDir) })

	log := logger.New("test")
	env, err := NewLocalEnvironment(tempDir, log)
	if err != nil {
		b.Fatal(err)
	}

	tool := NewWriteTool(env, nil)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tool.Execute(ctx, map[string]any{
			"file_path": fmt.Sprintf("bench_%d.txt", i),
			"content":   "Hello, World!",
		})
	}
}

// BenchmarkEditTool_SimpleReplacement 简单替换性能基准测试
func BenchmarkEditTool_SimpleReplacement(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "oxencode-bench-edit-*")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { os.RemoveAll(tempDir) })

	log := logger.New("test")
	env, err := NewLocalEnvironment(tempDir, log)
	if err != nil {
		b.Fatal(err)
	}

	tool := NewEditTool(env, nil)
	ctx := context.Background()

	// 准备测试文件
	testFile := "bench_edit.txt"
	_ = os.WriteFile(filepath.Join(env.GetWorkingDirectory(), testFile), []byte("Hello World\nHello World\nHello World\n"), 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tool.Execute(ctx, map[string]any{
			"file_path":  testFile,
			"old_string": "Hello",
			"new_string": "Hi",
		})
	}
}
