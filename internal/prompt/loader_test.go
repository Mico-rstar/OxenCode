package prompt

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoader_Load(t *testing.T) {
	// 创建临时测试目录
	tempDir := t.TempDir()

	// 创建主提示词文件
	mainContent := `# Test Prompt

{{INCLUDE:modules/test.md}}

<additional>
Additional content here
</additional>
`

	mainPath := filepath.Join(tempDir, "main_prompt.md")
	if err := os.WriteFile(mainPath, []byte(mainContent), 0644); err != nil {
		t.Fatalf("Failed to create main_prompt.md: %v", err)
	}

	// 创建模块目录和文件
	modulesDir := filepath.Join(tempDir, "modules")
	if err := os.MkdirAll(modulesDir, 0755); err != nil {
		t.Fatalf("Failed to create modules directory: %v", err)
	}

	moduleContent := `<test_module>
This is test module content
</test_module>`

	modulePath := filepath.Join(modulesDir, "test.md")
	if err := os.WriteFile(modulePath, []byte(moduleContent), 0644); err != nil {
		t.Fatalf("Failed to create test.md: %v", err)
	}

	// 测试加载器
	loader := NewLoader(tempDir)
	result, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// 验证结果包含模块内容
	if !contains(result, "<test_module>") {
		t.Errorf("Result does not contain module content")
	}
	if !contains(result, "This is test module content") {
		t.Errorf("Result does not contain module text")
	}
	if !contains(result, "Additional content here") {
		t.Errorf("Result does not contain additional content")
	}
}

func TestLoader_LoadRaw(t *testing.T) {
	tempDir := t.TempDir()

	mainContent := `# Test Prompt

{{INCLUDE:modules/test.md}}`

	mainPath := filepath.Join(tempDir, "main_prompt.md")
	if err := os.WriteFile(mainPath, []byte(mainContent), 0644); err != nil {
		t.Fatalf("Failed to create main_prompt.md: %v", err)
	}

	loader := NewLoader(tempDir)
	result, err := loader.LoadRaw()
	if err != nil {
		t.Fatalf("LoadRaw() failed: %v", err)
	}

	// LoadRaw 不应该解析 INCLUDE
	if !contains(result, "{{INCLUDE:modules/test.md}}") {
		t.Errorf("LoadRaw should not parse INCLUDE directives")
	}
}

func TestLoader_FileNotFound(t *testing.T) {
	loader := NewLoader("/nonexistent/path")
	_, err := loader.Load()
	if err == nil {
		t.Error("Expected error for nonexistent path, got nil")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
