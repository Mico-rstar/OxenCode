# L1 Compression Skill

## Target
Compress a single conversation turn into a structured summary that preserves:
- User's original request and underlying intent
- Agent's actions and tool usage
- Task completion status
- Key information (file changes, code snippets, errors, discoveries)

## Workflow

### Step 1: Extract User Input
- Capture the original user message (direct quote)
- Identify the user's true intent behind the request

### Step 2: Analyze Agent Actions
- List each tool call with brief description of what was done
- Summarize key results from tool executions
- Note the agent's final response to the user

### Step 3: Determine Task Status
- **COMPLETED**: Task finished successfully
- **IN_PROGRESS**: Work ongoing, more steps needed
- **BLOCKED**: Cannot proceed, needs clarification or unblocking
- **FAILED**: Task failed, error encountered

### Step 4: Extract Key Information
- **Files Modified**: List changed files with what changed
- **Key Code**: Important code snippets representing significant work
- **Findings**: Important discoveries, patterns, errors, insights
- **Dependencies**: New dependencies, imports, external references

## Output Schema

```markdown
# User Input
**Original Request:** [Direct quote or close paraphrase]
**Intent:** [What the user actually wanted to accomplish]

# Agent Actions
- **ToolName:** [Brief description of what was done and key result]
- **ToolName:** [Brief description of what was done and key result]
...

**Summary:** [What the agent communicated back to the user]

# Task Status
- **Status:** [COMPLETED | IN_PROGRESS | BLOCKED | FAILED]
- **Details:** [Brief explanation of current state]

# Key Information

## Files Modified
- [filename](path) - [what changed]

## Key Code
[filename:line](path) - [description]
[Code snippet if relevant]


## Findings
[Important discoveries, patterns, errors, or insights]

## Dependencies
[New dependencies, imports, or external references]
```

## Compression Guidelines

**Keep:**
- User's exact words when critical
- Specific file paths and line numbers
- Function names, variable names, API endpoints
- Error messages (essential parts only)
- Code snippets that represent meaningful changes
- Key decisions and trade-offs

**Discard:**
- Verbose tool output (logs, stack traces beyond error)
- Redundant explanations
- Confirmation messages
- Formatting noise

**Summarize:**
- Tool execution results → one-line description
- Multiple similar operations → consolidated single entry
- Long code blocks → representative snippet or description

## Examples

### Input: Tool Calls + Agent Response
```
User: "Fix the authentication bug in login.go"
Agent: [Read tool calls, Edit tool, tests run]
Agent: "Fixed the null pointer exception in login validation"
```

### Output (Compressed):
```markdown
# User Input
**Original Request:** Fix the authentication bug in login.go
**Intent:** Resolve bug preventing user login

# Agent Actions
- **Read:** Analyzed login.go:42-51 for bug
- **Edit:** Fixed null pointer exception in validateCredentials()
- **Bash:** Ran tests, all passed

**Summary:** Fixed null pointer exception in login validation

# Task Status
- **Status:** COMPLETED
- **Details:** Bug fixed, tests passing

# Key Information

## Files Modified
- [login.go:45](src/auth/login.go) - Added nil check for credentials

## Key Code
[login.go:45-47](src/auth/login.go) - Nil check implementation
```go
if credentials == nil {
    return errors.New("invalid credentials")
}
```

## Findings
- Root cause: Missing nil validation before accessing credential fields

## Dependencies
None
```
