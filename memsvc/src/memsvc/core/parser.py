"""Frontmatter parsing utilities for memory files."""

import re
from dataclasses import dataclass, field


@dataclass
class ParsedMemory:
    """Parsed memory file content."""

    description: str = ""
    body: str = ""
    metadata: dict = field(default_factory=dict)


# YAML frontmatter pattern: --- at start, content, --- at end
FRONTMATTER_PATTERN = re.compile(
    r"^---\s*\n(.*?)\n---\s*\n(.*)$",
    re.DOTALL,
)


def parse_frontmatter(frontmatter_text: str) -> dict:
    """Parse YAML-like frontmatter into a dictionary.

    Simple parser for key: value format without full YAML library.

    Args:
        frontmatter_text: Raw frontmatter text (between --- markers).

    Returns:
        Dictionary of parsed key-value pairs.
    """
    result = {}
    for line in frontmatter_text.strip().split("\n"):
        if ":" in line:
            key, value = line.split(":", 1)
            result[key.strip()] = value.strip()
    return result


def parse_memory_file(content: str) -> ParsedMemory:
    """Parse a memory file with optional frontmatter.

    Memory files use YAML-like frontmatter:
    ---
    description: A description of the content
    ---

    The actual content goes here...

    Args:
        content: Raw file content.

    Returns:
        ParsedMemory with description, body, and metadata.
    """
    match = FRONTMATTER_PATTERN.match(content)

    if match:
        frontmatter_text = match.group(1)
        body = match.group(2).strip()
        metadata = parse_frontmatter(frontmatter_text)
        description = metadata.get("description", "")
    else:
        # No frontmatter - entire content is body
        body = content.strip()
        metadata = {}
        description = ""

    return ParsedMemory(
        description=description,
        body=body,
        metadata=metadata,
    )


def extract_description(content: str) -> str:
    """Extract description from a memory file.

    Args:
        content: Raw file content.

    Returns:
        The description from frontmatter, or empty string if not found.
    """
    parsed = parse_memory_file(content)
    return parsed.description


def get_full_text_for_indexing(content: str) -> str:
    """Get full text for indexing (description + body).

    Args:
        content: Raw file content.

    Returns:
        Full text combining description and body.
    """
    parsed = parse_memory_file(content)
    if parsed.description and parsed.body:
        return f"{parsed.description}\n\n{parsed.body}"
    return parsed.description or parsed.body