package prompt

import (
	"reflect"
	"testing"
)

func TestExtractVariables(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected map[string]string
	}{
		{
			name:     "single tag",
			content:  "<identity>OxenCode AI</identity>",
			expected: map[string]string{"identity": "OxenCode AI"},
		},
		{
			name:     "multiple tags",
			content:  `<name>Test</name><version>1.0</version>`,
			expected: map[string]string{"name": "Test", "version": "1.0"},
		},
		{
			name:     "multiline content",
			content:  "<script>\nline1\nline2\n</script>",
			expected: map[string]string{"script": "\nline1\nline2\n"},
		},
		{
			name:     "nested tags - outer only",
			content:  "<outer><inner>value</inner></outer>",
			expected: map[string]string{"outer": "<inner>value</inner>"},
		},
		{
			name:     "no tags",
			content:  "plain text with no tags",
			expected: map[string]string{},
		},
		{
			name:     "duplicate tags - last wins",
			content:  "<val>first</val>...<val>second</val>",
			expected: map[string]string{"val": "second"},
		},
		{
			name:     "empty tag content",
			content:  "<empty></empty>",
			expected: map[string]string{"empty": ""},
		},
		{
			name:     "malformed tag - no closing",
			content:  "<open>no closing tag",
			expected: map[string]string{},
		},
		{
			name:     "mixed valid and invalid",
			content:  "<valid>content</valid><invalid>no closing",
			expected: map[string]string{"valid": "content"},
		},
		{
			name:     "tag with underscore",
			content:  "<my_var>value</my_var>",
			expected: map[string]string{"my_var": "value"},
		},
		{
			name:     "complex nested structure",
			content:  `<config><name>OxenCode</name><version>1.0</version></config>`,
			expected: map[string]string{"config": "<name>OxenCode</name><version>1.0</version>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractVariables(tt.content)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ExtractVariables() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestInjectVariables(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		vars     map[string]string
		expected string
	}{
		{
			name:     "single variable",
			content:  "Hello {{VAR:name}}",
			vars:     map[string]string{"name": "World"},
			expected: "Hello World",
		},
		{
			name:     "multiple variables",
			content:  "{{VAR:a}} and {{VAR:b}}",
			vars:     map[string]string{"a": "1", "b": "2"},
			expected: "1 and 2",
		},
		{
			name:     "missing variable - keep placeholder",
			content:  "Value: {{VAR:missing}}",
			vars:     map[string]string{},
			expected: "Value: {{VAR:missing}}",
		},
		{
			name:     "no variables",
			content:  "plain text",
			vars:     map[string]string{},
			expected: "plain text",
		},
		{
			name:     "nil vars map",
			content:  "{{VAR:test}}",
			vars:     nil,
			expected: "{{VAR:test}}",
		},
		{
			name:     "partial injection",
			content:  "{{VAR:a}} {{VAR:b}} {{VAR:c}}",
			vars:     map[string]string{"a": "1", "c": "3"},
			expected: "1 {{VAR:b}} 3",
		},
		{
			name:     "variable with special chars",
			content:  "{{VAR:text}}",
			vars:     map[string]string{"text": "hello\nworld\t!"},
			expected: "hello\nworld\t!",
		},
		{
			name:     "repeated variable",
			content:  "{{VAR:x}} and {{VAR:x}}",
			vars:     map[string]string{"x": "value"},
			expected: "value and value",
		},
		{
			name:     "complex template",
			content: `<config>
<name>{{VAR:name}}</name>
<version>{{VAR:version}}</version>
</config>`,
			vars:     map[string]string{"name": "OxenCode", "version": "1.0"},
			expected: `<config>
<name>OxenCode</name>
<version>1.0</version>
</config>`,
		},
		{
			name:     "variable with underscore",
			content:  "{{VAR:my_var}}",
			vars:     map[string]string{"my_var": "value"},
			expected: "value",
		},
		{
			name:     "mixed with INCLUDE directive",
			content:  "{{VAR:name}} {{INCLUDE:test.md}}",
			vars:     map[string]string{"name": "Test"},
			expected: "Test {{INCLUDE:test.md}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := InjectVariables(tt.content, tt.vars)
			if result != tt.expected {
				t.Errorf("InjectVariables() =\n%q\nwant:\n%q", result, tt.expected)
			}
		})
	}
}

func TestExtractAndInjectIntegration(t *testing.T) {
	source := `<model>gpt-4</model>
<timeout>30s</timeout>`

	template := `Model: {{VAR:model}}, Timeout: {{VAR:timeout}}`

	// Extract
	extracted := ExtractVariables(source)

	// Verify extraction
	if extracted["model"] != "gpt-4" {
		t.Errorf("Expected model=gpt-4, got %s", extracted["model"])
	}
	if extracted["timeout"] != "30s" {
		t.Errorf("Expected timeout=30s, got %q", extracted["timeout"])
	}

	// Inject
	result := InjectVariables(template, extracted)
	expected := "Model: gpt-4, Timeout: 30s"
	if result != expected {
		t.Errorf("InjectVariables() = %q, want %q", result, expected)
	}
}

func TestExtractVariablesEdgeCases(t *testing.T) {
	t.Run("self closing tag not supported", func(t *testing.T) {
		content := `<tag/>`
		result := ExtractVariables(content)
		if len(result) != 0 {
			t.Errorf("Self-closing tag should not match, got %v", result)
		}
	})

	t.Run("tag with attributes not supported", func(t *testing.T) {
		content := `<tag id="1">content</tag>`
		result := ExtractVariables(content)
		// 由于正则表达式不支持属性，这不应该匹配
		if len(result) != 0 {
			t.Errorf("Tag with attributes should not match, got %v", result)
		}
	})

	t.Run("multiple occurrences same tag", func(t *testing.T) {
		content := `<x>first</x> text <x>second</x>`
		result := ExtractVariables(content)
		if result["x"] != "second" {
			t.Errorf("Last occurrence should win, got %q", result["x"])
		}
	})

	t.Run("empty content returns empty map", func(t *testing.T) {
		result := ExtractVariables("")
		if len(result) != 0 {
			t.Errorf("Empty content should return empty map, got %v", result)
		}
	})
}

func TestInjectVariablesEdgeCases(t *testing.T) {
	t.Run("malformed placeholder kept as-is", func(t *testing.T) {
		content := "{{VAR:missing_bracket"
		result := InjectVariables(content, nil)
		if result != content {
			t.Errorf("Malformed placeholder should be kept, got %q", result)
		}
	})

	t.Run("case sensitive variable name", func(t *testing.T) {
		content := "{{VAR:Name}} {{VAR:name}}"
		vars := map[string]string{"name": "lower"}
		result := InjectVariables(content, vars)
		expected := "{{VAR:Name}} lower"
		if result != expected {
			t.Errorf("Case sensitivity failed, got %q, want %q", result, expected)
		}
	})

	t.Run("empty value in vars", func(t *testing.T) {
		content := "{{VAR:x}}"
		vars := map[string]string{"x": ""}
		result := InjectVariables(content, vars)
		if result != "" {
			t.Errorf("Empty value should replace placeholder, got %q", result)
		}
	})

	t.Run("special regex chars in value", func(t *testing.T) {
		content := "{{VAR:text}}"
		vars := map[string]string{"text": "$1.00 \\n \r\n"}
		result := InjectVariables(content, vars)
		if result != "$1.00 \\n \r\n" {
			t.Errorf("Special chars not handled correctly, got %q", result)
		}
	})
}

func TestExtractVariablesReturnsNewMap(t *testing.T) {
	content := "<name>Test</name>"
	result1 := ExtractVariables(content)
	result2 := ExtractVariables(content)

	// 两次调用应该返回不同的 map 实例
	if &result1 == &result2 {
		t.Error("ExtractVariables should return a new map each time")
	}

	// 修改一个不应该影响另一个
	result1["name"] = "Modified"
	if result2["name"] == "Modified" {
		t.Error("Modifying result should not affect subsequent calls")
	}
}
