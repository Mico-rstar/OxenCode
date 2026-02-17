package tools

import (
	"context"
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

// setupTestEnv 创建测试环境
func setupTestEnv(t *testing.T) (string, Environment) {
	tempDir, err := os.MkdirTemp("", "oxencode-p0test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	log := logger.New("test")
	env, err := NewLocalEnvironment(tempDir, log)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	return tempDir, env
}

// === Glob Tool Tests ===

func TestGlobTool(t *testing.T) {
	_, env := setupTestEnv(t)
	tool := NewGlobTool(env, nil)

	t.Run("Basic glob pattern", func(t *testing.T) {
		// 创建测试文件
		env.WriteFile("test1.txt", []byte("content1"), 0644)
		env.WriteFile("test2.txt", []byte("content2"), 0644)
		env.WriteFile("readme.md", []byte("markdown"), 0644)

		ctx := context.Background()
		result, err := tool.Execute(ctx, map[string]any{
			"pattern": "*.txt",
		})

		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		lines := strings.Split(result, "\n")
		if len(lines) < 2 {
			t.Errorf("Expected at least 2 .txt files, got %d: %v", len(lines), lines)
		}

		// 验证结果包含预期文件
		resultStr := strings.Join(lines, " ")
		if !strings.Contains(resultStr, "test1.txt") {
			t.Error("Result should contain test1.txt")
		}
		if !strings.Contains(resultStr, "test2.txt") {
			t.Error("Result should contain test2.txt")
		}
	})

	t.Run("Glob with path", func(t *testing.T) {
		// 创建子目录和文件
		subDir := "subdir"
		env.WriteFile(filepath.Join(subDir, "file.txt"), []byte("content"), 0644)

		ctx := context.Background()
		result, err := tool.Execute(ctx, map[string]any{
			"pattern": "*.txt",
			"path":    subDir,
		})

		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if !strings.Contains(result, "file.txt") {
			t.Errorf("Expected file.txt in result, got: %s", result)
		}
	})

	t.Run("Glob no matches", func(t *testing.T) {
		ctx := context.Background()
		result, err := tool.Execute(ctx, map[string]any{
			"pattern": "*.nonexistent",
		})

		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if result != "No matches found" {
			t.Errorf("Expected 'No matches found', got: %s", result)
		}
	})

	t.Run("Missing pattern parameter", func(t *testing.T) {
		ctx := context.Background()
		_, err := tool.Execute(ctx, map[string]any{})

		if err == nil {
			t.Error("Expected error for missing pattern parameter")
		}
	})
}

// === Grep Tool Tests ===

func TestGrepTool(t *testing.T) {
	_, env := setupTestEnv(t)
	tool := NewGrepTool(env, nil)

	t.Run("Basic grep", func(t *testing.T) {
		// 创建测试文件
		env.WriteFile("test.txt", []byte("hello world\nhello go\ngoodbye world"), 0644)

		ctx := context.Background()
		result, err := tool.Execute(ctx, map[string]any{
			"pattern": "hello",
		})

		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if !strings.Contains(result, "hello") {
			t.Errorf("Expected 'hello' in result, got: %s", result)
		}

		// 计算匹配行数
		lines := strings.Split(result, "\n")
		matchCount := 0
		for _, line := range lines {
			if strings.Contains(line, "hello") {
				matchCount++
			}
		}
		if matchCount != 2 {
			t.Errorf("Expected 2 matches, got %d", matchCount)
		}
	})

	t.Run("Grep with ignore case", func(t *testing.T) {
		env.WriteFile("case.txt", []byte("Hello\nHELLO\nhello"), 0644)

		ctx := context.Background()
		result, err := tool.Execute(ctx, map[string]any{
			"pattern":      "hello",
			"ignore_case":  true,
			"file_pattern": "case.txt",
		})

		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		lines := strings.Split(result, "\n")
		matchCount := 0
		for _, line := range lines {
			if strings.Contains(line, "hello") || strings.Contains(line, "Hello") || strings.Contains(line, "HELLO") {
				matchCount++
			}
		}
		if matchCount != 3 {
			t.Errorf("Expected 3 matches with ignore_case, got %d", matchCount)
		}
	})

	t.Run("Grep with file pattern", func(t *testing.T) {
		env.WriteFile("test.go", []byte("package main\nfunc main() {}"), 0644)
		env.WriteFile("test.txt", []byte("package main"), 0644)

		ctx := context.Background()
		result, err := tool.Execute(ctx, map[string]any{
			"pattern":      "package",
			"file_pattern": "*.go",
		})

		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// 应该只在 .go 文件中匹配
		if !strings.Contains(result, "test.go") {
			t.Error("Expected test.go in result")
		}
		if strings.Contains(result, "test.txt") {
			t.Error("Should not contain test.txt in result")
		}
	})

	t.Run("Grep no matches", func(t *testing.T) {
		env.WriteFile("empty.txt", []byte("some content here"), 0644)

		ctx := context.Background()
		result, err := tool.Execute(ctx, map[string]any{
			"pattern":      "nonexistent",
			"file_pattern": "empty.txt",
		})

		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if result != "No matches found" {
			t.Errorf("Expected 'No matches found', got: %s", result)
		}
	})

	t.Run("Invalid regex", func(t *testing.T) {
		ctx := context.Background()
		_, err := tool.Execute(ctx, map[string]any{
			"pattern": "[invalid", // 无效的正则表达式
		})

		if err == nil {
			t.Error("Expected error for invalid regex")
		}
	})
}

// === Read Tool Tests ===

func TestReadTool(t *testing.T) {
	_, env := setupTestEnv(t)
	tool := NewReadTool(env, nil)

	t.Run("Read entire file", func(t *testing.T) {
		content := "line1\nline2\nline3"
		env.WriteFile("test.txt", []byte(content), 0644)

		ctx := context.Background()
		result, err := tool.Execute(ctx, map[string]any{
			"file_path": "test.txt",
		})

		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if !strings.Contains(result, "line1") {
			t.Errorf("Expected 'line1' in result, got: %s", result)
		}
		if !strings.Contains(result, "line3") {
			t.Errorf("Expected 'line3' in result, got: %s", result)
		}

		// 验证行号存在
		if !strings.Contains(result, "→") {
			t.Error("Expected line numbers in result")
		}
	})

	t.Run("Read with offset", func(t *testing.T) {
		content := "line1\nline2\nline3\nline4\nline5"
		env.WriteFile("offset.txt", []byte(content), 0644)

		ctx := context.Background()
		result, err := tool.Execute(ctx, map[string]any{
			"file_path": "offset.txt",
			"offset":    3.0, // 从第3行开始
		})

		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// 应该从 line3 开始
		if !strings.Contains(result, "line3") {
			t.Errorf("Expected 'line3' in result, got: %s", result)
		}
		// 不应该包含 line1
		if strings.Contains(result, "line1") {
			t.Error("Should not contain 'line1' in result")
		}
	})

	t.Run("Read with limit", func(t *testing.T) {
		content := "line1\nline2\nline3\nline4\nline5"
		env.WriteFile("limit.txt", []byte(content), 0644)

		ctx := context.Background()
		result, err := tool.Execute(ctx, map[string]any{
			"file_path": "limit.txt",
			"limit":     2.0, // 只读取2行
		})

		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// 应该包含 line1 和 line2
		if !strings.Contains(result, "line1") {
			t.Error("Expected 'line1' in result")
		}
		if !strings.Contains(result, "line2") {
			t.Error("Expected 'line2' in result")
		}
		// 不应该包含 line3
		if strings.Contains(result, "line3") {
			t.Error("Should not contain 'line3' in result")
		}
	})

	t.Run("Read with offset and limit", func(t *testing.T) {
		content := "line1\nline2\nline3\nline4\nline5"
		env.WriteFile("both.txt", []byte(content), 0644)

		ctx := context.Background()
		result, err := tool.Execute(ctx, map[string]any{
			"file_path": "both.txt",
			"offset":    2.0, // 从第2行开始
			"limit":     2.0, // 读取2行
		})

		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// 应该包含 line2 和 line3
		if !strings.Contains(result, "line2") {
			t.Error("Expected 'line2' in result")
		}
		if !strings.Contains(result, "line3") {
			t.Error("Expected 'line3' in result")
		}
		// 不应该包含 line1 或 line4
		if strings.Contains(result, "line1") {
			t.Error("Should not contain 'line1' in result")
		}
		if strings.Contains(result, "line4") {
			t.Error("Should not contain 'line4' in result")
		}
	})

	t.Run("Read empty file", func(t *testing.T) {
		env.WriteFile("empty.txt", []byte(""), 0644)

		ctx := context.Background()
		result, err := tool.Execute(ctx, map[string]any{
			"file_path": "empty.txt",
		})

		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if result != "File is empty" {
			t.Errorf("Expected 'File is empty', got: %s", result)
		}
	})

	t.Run("Read nonexistent file", func(t *testing.T) {
		ctx := context.Background()
		_, err := tool.Execute(ctx, map[string]any{
			"file_path": "nonexistent.txt",
		})

		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})

	t.Run("Missing file_path parameter", func(t *testing.T) {
		ctx := context.Background()
		_, err := tool.Execute(ctx, map[string]any{})

		if err == nil {
			t.Error("Expected error for missing file_path parameter")
		}
	})

	t.Run("Offset exceeds file length", func(t *testing.T) {
		content := "line1\nline2"
		env.WriteFile("short.txt", []byte(content), 0644)

		ctx := context.Background()
		_, err := tool.Execute(ctx, map[string]any{
			"file_path": "short.txt",
			"offset":    10.0, // 超出文件长度
		})

		if err == nil {
			t.Error("Expected error for offset exceeding file length")
		}
	})
}
