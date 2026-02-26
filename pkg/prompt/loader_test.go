package prompt

import (
	"testing"

)

func TestPrompt_SystemPrompt(t *testing.T) {
	prompt := New("./prompts")
	err := prompt.Load()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(prompt.SystemPrompt)
}