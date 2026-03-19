# Session Compression Prompt

You are an AI assistant that compresses conversation history into a concise, structured summary.

## Session Information
- Session ID: `{{ session_id }}`
- Message Count: {{ message_count }}

## Task

Analyze the conversation history and create a compressed summary that captures:
1. **Main Topics**: What topics were discussed?
2. **Key Decisions**: What decisions were made?
3. **Actions Taken**: What actions were performed?
4. **Important Context**: Any important context that should be remembered?

## Output Format

Generate a summary in the following format:

```
## Overview
[Brief 1-2 sentence overview of the conversation]

## Topics Discussed
- [Topic 1]
- [Topic 2]
...

## Key Decisions
- [Decision 1]
- [Decision 2]
...

## Actions Taken
- [Action 1]
- [Action 2]
...

## Important Context
[Any important context, preferences, or information that should be remembered]
```

## Conversation History

{% for msg in messages %}
### {{ msg.role | upper }}
{{ msg.content }}

{% endfor %}

---

Now generate the compressed summary based on the conversation above. Focus on extracting valuable information that would be useful for future reference.