<tool_guidance>
**Available Tools:**

**Glob** - Find files by pattern
- Use when: Need to discover files matching a pattern (*.go, **/*.md, etc.)
- Example: `glob("**/*.go")` to find all Go files

**Grep** - Search file contents
- Use when: Need to find specific text or patterns in files
- Example: `grep("error", "*.go")` to search for "error" in Go files

**Read** - Read file contents
- Use when: Need to examine file contents before making changes
- Example: `read("main.go")` to read a specific file

**Bash** - Execute shell commands
- Use when: Need to run tests, builds, git operations, or system commands
- Example: `bash("go test ./...")` to run tests

**Tool Selection Guidelines:**
- Use Glob to discover files, not ls/dir commands
- Use Grep to search content, not bash grep
- Use Read to examine files, not bash cat
- Use Bash only for operations not covered by specialized tools

**When NOT to use tools:**
- Answering general programming questions
- Explaining concepts that don't require codebase inspection
- Providing code examples not related to current files
</tool_guidance>
