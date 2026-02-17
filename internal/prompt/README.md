# OxenCode Prompt System

This directory contains the system prompt definitions for the OxenCode Agent.

## Structure

```
internal/prompt/
├── main_prompt.md    # Main system prompt with INCLUDE directives
├── modules/          # Reusable prompt modules
│   ├── core.md       # Core principles (interaction, boundaries, ReAct)
│   └── tools.md      # Tool usage guidance
├── loader.go         # Prompt loader with INCLUDE processing
└── loader_test.go    # Tests for the loader
```

## Usage

### Automatic Loading via Agent (Recommended)

The Agent automatically loads the system prompt on initialization:

```go
import "github.com/yourname/oxencode/internal/agent"
import "github.com/yourname/oxencode/pkg/config"

// Load config (includes prompt_dir setting)
cfg, _ := config.Load()

// Create agent - automatically loads system prompt from cfg.PromptDir
ag, _ := agent.NewAgent(cfg)
```

**Configuration** (`~/.config/oxencode/config.toml`):

```toml
provider = "anthropic"
model = "claude-sonnet-4-5-20250514"
prompt_dir = "internal/prompt"  # Default: internal/prompt
work_dir = "."
```

### Dynamic Reload

Reload the system prompt at runtime without restarting:

```go
// Reload system prompt from configured directory
err := agent.ReloadSystemPrompt()
if err != nil {
    log.Fatal(err)
}
```

### Manual Loading

Load prompts programmatically for custom use:

```go
import "github.com/yourname/oxencode/internal/prompt"

// Create loader
loader := prompt.NewLoader("./internal/prompt")

// Load with INCLUDE processing
systemPrompt, err := loader.Load()
if err != nil {
    log.Fatal(err)
}

// Use directly
agent.SetSystemPrompt(systemPrompt)
```

### Quick Test

```bash
# Test the prompt loader
go test ./internal/prompt/... -v

# View the resolved prompt
go run internal/prompt/examples/loader/main.go

# Test agent with dynamic prompt loading
cd examples/agent_with_prompt && go run main.go
```

## Prompt Design Principles

This prompt system follows the **MVP philosophy**:

1. **Simplicity over completeness** - Focused on validating the tool system
2. **Modular for reusability** - Core principles and tools can be reused
3. **Concise for efficiency** - Minimal tokens, maximum clarity
4. **ReAct-oriented** - Explicit guidance for Thought-Action-Observation loops

## System Prompt Components

### `<identity>` - Core Role
Defines OxenCode as a tool-using AI programming assistant.

### `<core_principles>` - Behavior Guidelines
- **Interaction Style**: Direct, concise, reasoning-focused
- **Boundaries**: What the agent can/cannot do
- **ReAct Workflow**: How to use the Thought-Action-Observation pattern

### `<tool_guidance>` - Tool Instructions
- When to use each tool (Glob, Grep, Read, Bash)
- Tool selection priorities
- When NOT to use tools

### `<examples>` - Few-Shot Learning
Minimal examples demonstrating:
- Finding code in the codebase
- Running tests
- Using the ReAct pattern

## Customization

### Adding New Tools

1. Update `modules/tools.md` with the new tool description
2. Add usage examples to `<examples>` section if needed

### Modifying Behavior

- **Interaction style**: Edit `modules/core.md` → `<interaction_style>`
- **Boundaries**: Edit `modules/core.md` → `<boundaries>`
- **ReAct pattern**: Edit `modules/core.md` → `<react_workflow>`

### Adding New Modules

1. Create a new `.md` file in `modules/`
2. Reference it in `main_prompt.md`: `{{INCLUDE:modules/your_module.md}}`

## Token Count

Current estimated size:
- main_prompt.md: ~800 tokens (with includes resolved)
- Total with modules: ~1200 tokens

This fits well within the recommended limit for simple tool-using agents (< 2000 tokens).

## Future Improvements

As OxenCode grows beyond MVP:

1. Add Write/Edit tools to tool guidance
2. Add more complex examples
3. Consider creating specialized prompts for different tasks
4. Add prompt variants (e.g., for code review, debugging, etc.)
