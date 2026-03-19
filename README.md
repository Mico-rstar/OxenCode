<!-- NOTE: This file is synchronized with README.zh-CN.md. When updating, please update both files. -->

<div align="center">

# 🐂 OxenCode

**An AI Programming Assistant with Innovative Context Management**

[English](#readme) | [简体中文](README.zh-CN.md)

</div>

---

## Overview

OxenCode is a learning-oriented AI programming assistant built with Go. It features a unique architecture that separates agent execution from tool execution, and implements an innovative context management system that enables long-running tasks without hitting model context limits.

> **NOTE:** This is a learning project focused on Agent engineering best practices and innovative context management strategies.

## Key Features

### 🏗️ Agent/Tool Environment Separation

OxenCode's core innovation is the separation of agent execution and tool execution environments. This design provides:

- **Enhanced Security**: Tools run in isolated environments, preventing unintended side effects
- **Improved Reliability**: Tool failures don't crash the agent process
- **Better Resource Management**: Separate resource limits for agent reasoning and tool execution

### ⏱️ Long-Running Task Support

Unlike traditional AI assistants that fail when context exceeds model limits, OxenCode handles extended tasks through:

- **Multi-batch Context Compression**: Asynchronous, user-transparent context management
- **Context Hierarchies**: L0 > L1 > L2 compression levels for optimal performance
- **Session Isolation**: Task contexts are isolated to prevent interference
- **Stable Prefix Design**: High cache hit rates for efficient token usage

### 🔄 ReAct Loop Implementation

OxenCode implements the ReAct (Reasoning + Acting) pattern for iterative problem-solving:

- **Thought → Action → Observation** cycle for complex tasks
- **Stream Processing**: Real-time display of AI reasoning and tool execution
- **Error Recovery**: Automatic retry and strategy adjustment on failures
- **LLM Reasoning Display**: Shows model thinking process for supported models

### 🛠️ Rich Tool Ecosystem

Built-in tools for common development tasks:

- **File Operations**: `Glob`, `Grep`, `Read`, `Write`, `Edit`
- **System Commands**: `Bash` with configurable timeout
- **Permission System**: User authorization for dangerous operations
- **Smart Validation**: Parameter validation before tool execution

### 🌐 Multi-Provider Support

Support for multiple LLM providers:

- **Anthropic**: Claude (Sonnet, Haiku) with extended thinking
- **OpenAI**: GPT-4, GPT-4o, o1 series
- **Google**: Gemini 2.0 Flash, Gemini 1.5 Pro
- **Qwen**: Qwen-Max, Qwen-Plus, Qwen-Turbo
- **DeepSeek**: DeepSeek-Chat, DeepSeek-Coder, DeepSeek-Reasoner
- **GLM**: GLM-4 series
- **Azure OpenAI**, **AWS Bedrock**, **OpenRouter**, and more

## Quick Start

### Prerequisites

- Go 1.25 or later
- An API key for your preferred LLM provider

### Installation

```bash
# Clone the repository
git clone https://github.com/yourname/oxencode.git
cd oxencode

# Build the project
go build -o oxencode ./cmd/oxencode

# (Optional) Install to system
go install ./cmd/oxencode
```

### Configuration

1. Copy the example configuration:

```bash
mkdir -p ~/.oxencode
cp config.example.toml ~/.oxencode/config.toml
```

2. Edit `~/.oxencode/config.toml` with your settings:

```toml
# Choose your provider
provider = "anthropic"  # or "openai", "deepseek", "qwen", etc.

# Set your model
model = "claude-sonnet-4-5-20250514"

# Configure work directory (optional)
work_dir = "."  # Current directory

# Set tool timeout (optional)
tool_timeout = 120  # seconds
```

3. Set your API key as an environment variable:

```bash
# For Anthropic Claude
export ANTHROPIC_API_KEY="your-key-here"

# For OpenAI
export OPENAI_API_KEY="your-key-here"

# For DeepSeek
export DEEPSEEK_API_KEY="your-key-here"
```

### Run OxenCode

```bash
./oxencode
```

## Usage Examples

### Basic Conversation

```
You: What files are in this directory?

OxenCode: [Uses Glob tool to list files]
```

### Code Analysis

```
You: Find all Go files that contain "error" and show me the first one.

OxenCode: [Uses Grep to find files, then Read to display content]
```

### Multi-Step Tasks

```
You: Create a simple HTTP server in Go that responds "Hello, World"

OxenCode: [Uses Write tool to create main.go with server code]
```

### Interrupting Tasks

Press `Esc` at any time to interrupt a running task.

## Documentation

For detailed documentation, see:

- [Architecture Overview](docs/architecture.md) - System design and data flow
- [ReAct Loop Implementation](docs/react-loop.md) - How the reasoning cycle works
- [Tool Integration](docs/tool-integration.md) - Building and registering tools
- [Tool Behavior Reference](docs/tools-behavior.md) - Available tools and their usage
- [TUI Testing](docs/tui-testing.md) - Testing the terminal UI

## Configuration Reference

Full configuration options are documented in [config.example.toml](config.example.toml).

Key configuration areas:

- **Provider Selection**: Choose from 10+ LLM providers
- **Model Settings**: Model selection, temperature, max tokens
- **Extended Thinking**: Enable reasoning for supported models
- **Work Directory**: Where tools operate
- **Tool Timeout**: Safety limit for tool execution

## Contributing

Contributions are welcome! This is a learning project, so feel free to:

- Report bugs
- Suggest new features
- Submit pull requests
- Improve documentation
- Share your usage patterns

When contributing, please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## Architecture Highlights

OxenCode uses a layered architecture:

```
┌─────────────────────────────────────┐
│     Presentation Layer (TUI)       │
│         Bubble Tea UI              │
└─────────────────────────────────────┘
                ↓
┌─────────────────────────────────────┐
│     Application Layer              │
│    Chat / Tool / Auth Managers     │
└─────────────────────────────────────┘
                ↓
┌─────────────────────────────────────┐
│     Domain Layer                   │
│  Agent (ReAct) + Tools + Permissions│
│         Fantasy SDK                │
└─────────────────────────────────────┘
                ↓
┌─────────────────────────────────────┐
│   Infrastructure Layer             │
│  File System / Config / History    │
└─────────────────────────────────────┘
```

## Roadmap

- [ ] Enhanced context compression algorithms
- [ ] Parallel tool execution
- [ ] Tool result caching
- [ ] Interactive debugging mode
- [ ] Plugin system for custom tools
- [ ] Multi-language support in UI

## License

MIT License - see LICENSE file for details

## Acknowledgments

- [fantasy](https://github.com/charmbracelet/fantasy) - Go SDK for LLM interaction
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Terminal UI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Style and formatting
- All open-source contributors making AI tools accessible

---

<div align="center">

**Built with ❤️ for learning Agent engineering**

[↑ Back to top](#-oxencode)

</div>
