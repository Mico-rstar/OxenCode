"""File tools for memory agents with path sandboxing."""

import re
from pathlib import Path
from typing import Callable

from langchain_core.tools import tool

from memsvc.config import settings


def create_memory_tools(
    write_dir: str,
    memory_dir: Path | None = None,
) -> list[Callable]:
    """Create file tools with write access restricted to a specific directory.

    Agents can read from anywhere in memory/ but can only write to their
    assigned directory (experience, knowledge, or inner).

    Args:
        write_dir: Directory the agent can write to (e.g., 'experience', 'knowledge', 'inner')
        memory_dir: Root memory directory. Defaults to settings.memory_dir.

    Returns:
        List of tool functions for the agent.
    """
    memory_dir = memory_dir or settings.memory_dir
    allowed_write_path = memory_dir / write_dir

    def validate_read_path(path: str) -> tuple[Path, str | None]:
        """Validate read path is within memory directory.

        Returns:
            Tuple of (full_path, error_message or None)
        """
        full_path = (memory_dir / path).resolve()
        if not full_path.is_relative_to(memory_dir):
            return full_path, "Error: Access denied - path outside memory directory"
        return full_path, None

    def validate_write_path(path: str) -> tuple[Path, str | None]:
        """Validate write path is within allowed write directory.

        Returns:
            Tuple of (full_path, error_message or None)
        """
        full_path = (allowed_write_path / path).resolve()
        if not full_path.is_relative_to(allowed_write_path):
            return full_path, f"Error: Cannot write outside {write_dir} directory"
        return full_path, None

    @tool
    def read_file(path: str) -> str:
        """Read a file from the memory directory.

        Use this to read notes, existing memories, or any file in memory/.

        Args:
            path: Relative path from memory directory (e.g., 'notes/session_id.md')

        Returns:
            File content or error message.
        """
        full_path, error = validate_read_path(path)
        if error:
            return error
        if not full_path.exists():
            return f"Error: File not found: {path}"
        try:
            return full_path.read_text(encoding="utf-8")
        except Exception as e:
            return f"Error reading file: {e}"

    @tool
    def write_file(path: str, content: str) -> str:
        """Write content to a new file in the agent's write directory.

        The file will be created in {write_dir}/{path}.
        Use this to create new memory files.

        Args:
            path: Filename or relative path (e.g., 'new_experience.md')
            content: Full file content including frontmatter

        Returns:
            Success message or error.
        """
        full_path, error = validate_write_path(path)
        if error:
            return error

        if full_path.exists():
            return f"Error: File already exists: {path}. Use edit_file to modify."

        try:
            full_path.parent.mkdir(parents=True, exist_ok=True)
            full_path.write_text(content, encoding="utf-8")
            return f"Successfully wrote to {write_dir}/{path}"
        except Exception as e:
            return f"Error writing file: {e}"

    @tool
    def edit_file(path: str, old_content: str, new_content: str) -> str:
        """Edit an existing file in the agent's write directory.

        Use this to update existing memory files like inner/self.md.

        Args:
            path: Filename to edit (e.g., 'self.md' for inner agent)
            old_content: Exact text to find and replace
            new_content: New text to insert in place of old_content

        Returns:
            Success message or error.
        """
        full_path, error = validate_write_path(path)
        if error:
            return error

        if not full_path.exists():
            return f"Error: File not found: {path}"

        try:
            content = full_path.read_text(encoding="utf-8")
            if old_content not in content:
                return "Error: old_content not found in file. Use read_file to see current content."

            updated = content.replace(old_content, new_content, 1)
            full_path.write_text(updated, encoding="utf-8")
            return f"Successfully edited {write_dir}/{path}"
        except Exception as e:
            return f"Error editing file: {e}"

    @tool
    def list_files(directory: str = "") -> str:
        """List markdown files in a directory.

        Args:
            directory: Relative path from memory directory (e.g., 'experience' or '' for root)

        Returns:
            List of .md files or error message.
        """
        full_path, error = validate_read_path(directory)
        if error:
            return error

        if not full_path.exists():
            return f"Directory not found: {directory}"

        files = sorted(full_path.glob("*.md"))
        if not files:
            return f"No .md files in {directory}"

        return "\n".join(f"- {f.name}" for f in files)

    @tool
    def grep_files(pattern: str, directory: str = "") -> str:
        """Search for a text pattern in files.

        Returns matching lines with file paths and line numbers.

        Args:
            pattern: Text or regex pattern to search for
            directory: Directory to search (relative to memory, empty for all)

        Returns:
            Matching lines or "No matches found".
        """
        full_path, error = validate_read_path(directory)
        if error:
            return error

        try:
            regex = re.compile(pattern, re.IGNORECASE)
        except re.error:
            regex = re.compile(re.escape(pattern), re.IGNORECASE)

        results = []
        for md_file in sorted(full_path.rglob("*.md")):
            try:
                content = md_file.read_text(encoding="utf-8")
                for i, line in enumerate(content.split("\n"), 1):
                    if regex.search(line):
                        rel_path = md_file.relative_to(memory_dir)
                        results.append(f"{rel_path}:{i}: {line.strip()[:100]}")
                        if len(results) >= 30:
                            return "\n".join(results) + "\n... (truncated)"
            except Exception:
                continue

        return "\n".join(results) if results else "No matches found"

    @tool
    def file_exists(path: str) -> str:
        """Check if a file exists.

        Args:
            path: Relative path from memory directory

        Returns:
            "exists" or "not_found".
        """
        full_path, error = validate_read_path(path)
        if error:
            return error

        return "exists" if full_path.exists() else "not_found"

    @tool
    def head_file(path: str, lines: int = 20) -> str:
        """Read the first N lines of a file.

        Use this to quickly preview the beginning of a file without reading the entire content.

        Args:
            path: Relative path from memory directory (e.g., 'notes/session_id.md')
            lines: Number of lines to read from the beginning (default: 20)

        Returns:
            File content (first N lines with line numbers) or error message.
        """
        full_path, error = validate_read_path(path)
        if error:
            return error
        if not full_path.exists():
            return f"Error: File not found: {path}"
        try:
            content = full_path.read_text(encoding="utf-8")
            file_lines = content.split("\n")
            head_lines = file_lines[:lines]
            result = []
            for i, line in enumerate(head_lines, 1):
                result.append(f"{i:5d}→{line}")
            if len(file_lines) > lines:
                result.append(f"... ({len(file_lines) - lines} more lines)")
            return "\n".join(result)
        except Exception as e:
            return f"Error reading file: {e}"

    @tool
    def tail_file(path: str, lines: int = 20) -> str:
        """Read the last N lines of a file.

        Use this to quickly preview the end of a file without reading the entire content.

        Args:
            path: Relative path from memory directory (e.g., 'notes/session_id.md')
            lines: Number of lines to read from the end (default: 20)

        Returns:
            File content (last N lines with line numbers) or error message.
        """
        full_path, error = validate_read_path(path)
        if error:
            return error
        if not full_path.exists():
            return f"Error: File not found: {path}"
        try:
            content = full_path.read_text(encoding="utf-8")
            file_lines = content.split("\n")
            tail_lines = file_lines[-lines:] if len(file_lines) > lines else file_lines
            start_line = max(1, len(file_lines) - lines + 1)
            result = []
            for i, line in enumerate(tail_lines, start_line):
                result.append(f"{i:5d}→{line}")
            if len(file_lines) > lines:
                result.insert(0, f"... ({len(file_lines) - lines} lines omitted)")
            return "\n".join(result)
        except Exception as e:
            return f"Error reading file: {e}"

    return [read_file, write_file, edit_file, list_files, grep_files, file_exists, head_file, tail_file]