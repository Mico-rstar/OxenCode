"""Session compressor for converting messages to notes."""

import json
import logging
from datetime import datetime
from pathlib import Path

from memsvc.config import settings
from memsvc.core.llm import QwenLLM, MockLLM
from memsvc.models.session import SessionTask, SessionData, Message
from memsvc.utils.prompt_loader import PromptLoader

logger = logging.getLogger(__name__)


class SessionCompressor:
    """Compresses session messages into notes.

    Handles:
    1. Writing messages to histories/{session_id}.json
    2. Compressing messages to notes/{session_id}.md using LLM
    """

    def __init__(
        self,
        llm: QwenLLM | MockLLM,
        memory_dir: Path | None = None,
        prompt_loader: PromptLoader | None = None,
    ):
        """Initialize session compressor.

        Args:
            llm: LLM provider for compression.
            memory_dir: Memory directory path. Defaults to settings.memory_dir.
            prompt_loader: Prompt loader. Defaults to one with effective_prompts_dir.
        """
        self.llm = llm
        self.memory_dir = memory_dir or settings.memory_dir
        self.prompt_loader = prompt_loader or PromptLoader(settings.effective_prompts_dir)

        # Ensure directories exist
        self.histories_dir = self.memory_dir / "histories"
        self.notes_dir = self.memory_dir / "notes"
        self.histories_dir.mkdir(parents=True, exist_ok=True)
        self.notes_dir.mkdir(parents=True, exist_ok=True)

    async def write_histories(
        self,
        session_id: str,
        messages: list[dict],
    ) -> Path:
        """Write messages to histories file.

        Args:
            session_id: Session identifier.
            messages: List of message dicts with 'role' and 'content'.

        Returns:
            Path to the histories file.
        """
        histories_path = self.histories_dir / f"{session_id}.json"

        # Create session data
        session_data = {
            "session_id": session_id,
            "messages": messages,
            "created_at": datetime.now().isoformat(),
        }

        # Write to file
        histories_path.write_text(
            json.dumps(session_data, indent=2, ensure_ascii=False),
            encoding="utf-8"
        )

        logger.info(f"Wrote histories for session {session_id}: {len(messages)} messages")
        return histories_path

    async def compress_to_notes(
        self,
        session_id: str,
        messages: list[dict],
    ) -> Path:
        """Compress messages to notes file using LLM.

        Args:
            session_id: Session identifier.
            messages: List of message dicts with 'role' and 'content'.

        Returns:
            Path to the notes file.
        """
        notes_path = self.notes_dir / f"{session_id}.md"

        # Load compression prompt
        prompt = self.prompt_loader.load(
            "session_compress",
            session_id=session_id,
            message_count=len(messages),
            messages=messages,
        )

        # Call LLM for compression
        logger.info(f"Compressing session {session_id} with {len(messages)} messages")
        compressed_content = await self.llm.complete(prompt)

        # Generate description from first few lines of content
        lines = compressed_content.strip().split("\n")
        description = ""
        for line in lines:
            # Skip empty lines and markdown headers
            line = line.strip()
            if line and not line.startswith("#"):
                description = line[:200]  # Max 200 chars for description
                break

        if not description:
            description = f"Session {session_id} summary"

        # Create frontmatter with source_history reference
        created_at = datetime.now().strftime("%Y-%m-%dT%H:%M:%S")
        histories_filename = f"{session_id}.json"
        frontmatter = f"""---
description: {description}
created_at: {created_at}
source_history: histories/{histories_filename}
---

"""

        # Write notes file
        notes_path.write_text(
            frontmatter + compressed_content,
            encoding="utf-8"
        )

        logger.info(f"Wrote notes for session {session_id}")
        return notes_path

    async def process_session(
        self,
        task: SessionTask,
        messages: list[dict],
        skip_histories: bool = False,
    ) -> tuple[Path | None, Path]:
        """Process a complete session: write histories and compress to notes.

        Args:
            task: Session task being processed.
            messages: List of message dicts.
            skip_histories: If True, skip writing histories (for retry).

        Returns:
            Tuple of (histories_path, notes_path).
        """
        histories_path = None

        # Step 1: Write histories (unless skipped)
        if not skip_histories:
            histories_path = await self.write_histories(task.session_id, messages)
            task.histories_written = True
        else:
            logger.info(f"Skipping histories for session {task.session_id} (retry)")

        # Step 2: Compress to notes
        notes_path = await self.compress_to_notes(task.session_id, messages)

        return histories_path, notes_path

    def load_histories(self, session_id: str) -> dict | None:
        """Load histories file for a session.

        Args:
            session_id: Session identifier.

        Returns:
            Session data dict or None if not found.
        """
        histories_path = self.histories_dir / f"{session_id}.json"
        if not histories_path.exists():
            return None

        try:
            data = json.loads(histories_path.read_text(encoding="utf-8"))
            return data
        except Exception as e:
            logger.error(f"Failed to load histories for {session_id}: {e}")
            return None

    def load_notes(self, session_id: str) -> str | None:
        """Load notes file for a session.

        Args:
            session_id: Session identifier.

        Returns:
            Notes content or None if not found.
        """
        notes_path = self.notes_dir / f"{session_id}.md"
        if not notes_path.exists():
            return None

        try:
            return notes_path.read_text(encoding="utf-8")
        except Exception as e:
            logger.error(f"Failed to load notes for {session_id}: {e}")
            return None

    def histories_exists(self, session_id: str) -> bool:
        """Check if histories file exists."""
        return (self.histories_dir / f"{session_id}.json").exists()

    def notes_exists(self, session_id: str) -> bool:
        """Check if notes file exists."""
        return (self.notes_dir / f"{session_id}.md").exists()