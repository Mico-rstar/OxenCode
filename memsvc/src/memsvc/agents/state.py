"""State definitions for session processing workflow."""

from datetime import datetime
from typing import Annotated, TypedDict
import operator


class AgentResult(TypedDict):
    """Result from a single memory agent."""
    agent_name: str
    success: bool
    files_written: list[str]
    files_edited: list[str]
    error: str | None


class SessionWorkflowState(TypedDict):
    """State for the entire session processing workflow.

    This state flows through all steps from histories to re_embed.

    Attributes:
        session_id: Unique session identifier
        messages: List of message dicts with 'role' and 'content'
        histories_written: Whether histories file was written
        histories_path: Path to histories JSON file
        notes_written: Whether notes file was written
        notes_path: Path to notes markdown file
        notes_content: Full content of the compressed notes
        existing_experiences: Summary of existing experience memories
        existing_knowledge: Summary of existing knowledge memories
        current_self: Current content of inner/self.md
        current_user: Current content of inner/user.md
        agent_results: Results from parallel agent execution (aggregated)
        re_embed_done: Whether re-embedding was completed
        files_indexed: List of files that were indexed
        started_at: Workflow start timestamp
        completed_at: Workflow completion timestamp
        error: Error message if workflow failed
        retry_count: Number of retry attempts
    """
    # Input
    session_id: str
    messages: list[dict]

    # Step 1: Write histories
    histories_written: bool
    histories_path: str | None

    # Step 2: Compress notes
    notes_written: bool
    notes_path: str | None
    notes_content: str

    # Step 3: Agent context (loaded before parallel execution)
    current_self: str
    current_user: str

    # Step 3: Agent results (parallel aggregation)
    agent_results: Annotated[list[AgentResult], operator.add]

    # Step 4: Re-embed
    re_embed_done: bool
    files_indexed: list[str]

    # Metadata
    started_at: datetime
    completed_at: datetime | None
    error: str | None
    retry_count: int