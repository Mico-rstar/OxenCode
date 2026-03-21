**search_memory** - Search long-term memory for relevant context
- Use when: SystemReminder indicates relevant memories exist
- Use when: Need historical experience, knowledge, or past session context
- Example: `search_memory(queries=["git conflict resolution", "merge branch"], top_k=5)`

**load_memory** - Load full content of specific memories
- Use when: Need complete details after search_memory returns results
- Example: `load_memory(ids=["exp_001", "know_003"])`

**Pattern:** Use multiple queries in one search call, then load only the most relevant results.

## Examples

**Example: Debugging with memory**
```
SystemReminder: Found relevant memories about "database connection timeout" (score: 0.82)

Action: search_memory(queries=["database timeout", "connection pool", "retry logic"], top_k=5)

Observation:
1. [exp_012] (0.89) - Handled PostgreSQL timeout by increasing pool size
2. [know_007] (0.76) - Connection pool configured in config/database.yml
3. [exp_003] (0.71) - Retry pattern for transient DB errors

Action: load_memory(ids=["exp_012", "exp_003"])

Observation: [Full content of selected memories...]

Response: "Based on past experience, I increased the pool size and added retry logic..."
```

**Example: Learning project conventions**
```
User: "Add a new API endpoint"

Action: search_memory(queries=["API endpoint pattern", "handler structure", "error handling"], top_k=5)

Observation:
1. [exp_021] (0.91) - API endpoints follow REST pattern with middleware
2. [know_015] (0.84) - All handlers in internal/api/ directory

Action: load_memory(ids=["exp_021"])

Observation: [Details about REST pattern and middleware chain...]

Response: "I'll follow our established pattern: create handler in internal/api/, add middleware..."
```

