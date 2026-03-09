# L0 Compression Skill

## Target
Compress multiple conversation turns into a high-level project summary that preserves:
- Project context and background
- Cumulative user requests and intents
- Key achievements and progress
- Critical information for context continuity

## Workflow

### Step 1: Analyze Project Context
- Identify project type, tech stack, and overall structure
- Note any architectural patterns or conventions

### Step 2: Summarize User Inputs
- List all user requests chronologically
- Identify the underlying intent behind each request
- Note any recurring themes or priorities

### Step 3: Consolidate Achievements
- Summarize completed work across all turns
- Highlight major milestones and progress
- Note any partial or ongoing work

### Step 4: Extract Key Information
- **Core Structures**: Key files, modules, and their relationships
- **Code Patterns**: Notable patterns, conventions, style observed
- **Issues & Resolutions**: Important bugs, errors, and how they were solved
- **Dependencies**: New dependencies, imports, external references

### Step 5: Capture Active State
- What was the agent working on most recently
- What's in progress or pending
- What's the logical next step

## Output Schema

```markdown
# Project Context
[Project background, tech stack, overall structure]

# User Inputs Summary
1. [Request 1 - brief description]
2. [Request 2 - brief description]
...

# Key Achievements
[Major completed work, cumulative progress]

# Key Information

## Core Structures
[Key files, modules, and their relationships]

## Code Patterns
[Notable patterns, conventions, style observed]

## Issues & Resolutions
[Important bugs, errors, and how they were solved]

# Core Concepts
[Domain concepts, keywords with brief explanations]

# Active State
[What the agent was working on, what's in progress, what's next]

# Important Notes
[Constraints, configurations, special considerations]
```

## Compression Guidelines

**Keep:**
- User's core intent across all interactions
- Specific file paths and line numbers
- Function names, variable names, API endpoints
- Key decisions and trade-offs
- Critical errors and their resolutions

**Discard:**
- Verbose tool outputs (keep only essential results)
- Redundant explanations
- Confirmation messages
- Formatting noise
- Detailed step-by-step logs

**Summarize:**
- Multiple similar operations → consolidated single entry
- Long code blocks → representative snippet or description
- Repetitive tool calls → one-line summary

## Examples

### Input: Multiple L1 Pages
```
L1 Page 1: User asked to implement authentication, agent created login.go, added middleware...
L1 Page 2: User asked to add password reset, agent modified login.go, created reset.go...
L1 Page 3: User asked to add rate limiting, agent created limiter.go, modified middleware...
```

### Output (Compressed):
```markdown
# Project Context
Authentication system for a Go web application using middleware pattern.

# User Inputs Summary
1. Implement user authentication
2. Add password reset functionality
3. Implement rate limiting

# Key Achievements
- Complete authentication system with login/logout
- Password reset with email verification
- Rate limiting middleware (100 req/min per IP)

# Key Information

## Core Structures
- [login.go](src/auth/login.go) - Core authentication logic
- [middleware.go](src/auth/middleware.go) - Auth middleware
- [reset.go](src/auth/reset.go) - Password reset flow
- [limiter.go](src/auth/limiter.go) - Rate limiting

## Code Patterns
- Middleware chain pattern for auth checks
- JWT tokens with 24h expiry
- Rate limiting using token bucket algorithm

## Issues & Resolutions
- Fixed nil pointer in validateCredentials() - added nil check
- Rate limiter memory leak - implemented cleanup goroutine

# Core Concepts
- JWT: JSON Web Tokens for stateless auth
- Token Bucket: Rate limiting algorithm

# Active State
Last working on: Rate limiting implementation
Status: Completed, tests passing
Next: User requested session management

# Important Notes
- All auth tokens expire in 24h
- Rate limit: 100 req/min per IP
- Using bcrypt for password hashing
```