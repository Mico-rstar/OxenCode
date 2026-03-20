"""Task manager for async session processing."""

import asyncio
import logging
import uuid
from datetime import datetime
from typing import Callable, Awaitable, AsyncIterator, TYPE_CHECKING

import aiosqlite

from memsvc.config import settings
from memsvc.core.metadata import MetadataManager
from memsvc.models.session import SessionTask, TaskStatus

if TYPE_CHECKING:
    from memsvc.agents.workflow import SessionWorkflow

logger = logging.getLogger(__name__)


class TaskManager:
    """Manages async session processing tasks.

    Uses a FIFO queue for sequential task execution.
    Tasks are persisted in SQLite for recovery on restart.
    """

    CREATE_TABLE_SQL = """
    CREATE TABLE IF NOT EXISTS session_tasks (
        task_id TEXT PRIMARY KEY,
        session_id TEXT NOT NULL UNIQUE,
        status TEXT NOT NULL,
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL,
        error_message TEXT,
        histories_written INTEGER DEFAULT 0
    )
    """

    CREATE_INDEX_SQL = """
    CREATE INDEX IF NOT EXISTS idx_session_tasks_status ON session_tasks(status)
    """

    CREATE_INDEX_SESSION_SQL = """
    CREATE INDEX IF NOT EXISTS idx_session_tasks_session_id ON session_tasks(session_id)
    """

    def __init__(self, metadata_manager: MetadataManager):
        """Initialize task manager.

        Args:
            metadata_manager: Metadata manager for database access.
        """
        self.metadata_manager = metadata_manager
        self._queue: asyncio.Queue[SessionTask] = asyncio.Queue()
        self._worker_task: asyncio.Task | None = None
        self._tasks: dict[str, SessionTask] = {}  # task_id -> SessionTask
        self._processor: Callable[[SessionTask, list], Awaitable[None]] | None = None
        self._workflow: "SessionWorkflow | None" = None
        self._pending_messages: dict[str, list] = {}  # session_id -> messages
        self._db: aiosqlite.Connection | None = None

    async def initialize(self) -> None:
        """Initialize database and start worker."""
        # Create session_tasks table
        if self.metadata_manager._db:
            await self.metadata_manager._db.execute(self.CREATE_TABLE_SQL)
            await self.metadata_manager._db.execute(self.CREATE_INDEX_SQL)
            await self.metadata_manager._db.execute(self.CREATE_INDEX_SESSION_SQL)
            await self.metadata_manager._db.commit()

        # Load pending/running tasks from database
        await self._load_pending_tasks()

        # Start worker
        self._worker_task = asyncio.create_task(self._worker())
        logger.info("Task manager initialized")

    async def close(self) -> None:
        """Stop worker and cleanup."""
        if self._worker_task:
            self._worker_task.cancel()
            try:
                await self._worker_task
            except asyncio.CancelledError:
                pass
            self._worker_task = None
        logger.info("Task manager closed")

    def set_processor(
        self,
        processor: Callable[[SessionTask, list], Awaitable[None]]
    ) -> None:
        """Set the task processor function.

        The processor will be called for each task with:
        - task: The SessionTask to process
        - messages: The messages for this session

        Args:
            processor: Async function to process tasks.
        """
        self._processor = processor
        logger.debug("Task processor set")

    def set_workflow(self, workflow: "SessionWorkflow") -> None:
        """Set the LangGraph workflow for processing.

        When a workflow is set, it takes precedence over the processor callback.

        Args:
            workflow: SessionWorkflow instance for LangGraph-based processing.
        """
        self._workflow = workflow
        logger.debug("SessionWorkflow set")

    async def create_task(
        self,
        session_id: str,
        messages: list,
    ) -> str:
        """Create a new session processing task.

        Args:
            session_id: Unique session identifier.
            messages: List of messages to process.

        Returns:
            task_id: Unique task identifier.

        Raises:
            ValueError: If session already has a pending/running task.
        """
        # Check for existing task
        existing = await self.get_task_by_session(session_id)
        if existing and existing.status in (TaskStatus.PENDING, TaskStatus.RUNNING):
            raise ValueError(f"Session {session_id} already has a {existing.status} task")

        # Create task
        task_id = f"task_{uuid.uuid4().hex[:12]}"
        now = datetime.now()
        task = SessionTask(
            task_id=task_id,
            session_id=session_id,
            status=TaskStatus.PENDING,
            created_at=now,
            updated_at=now,
        )

        # Store messages for processing
        self._pending_messages[session_id] = messages

        # Persist to database
        await self._save_task(task)

        # Add to in-memory tracking
        self._tasks[task_id] = task

        # Queue for processing
        await self._queue.put(task)

        logger.info(f"Created task {task_id} for session {session_id}")
        return task_id

    async def get_task(self, task_id: str) -> SessionTask | None:
        """Get task by task_id.

        Args:
            task_id: Task identifier.

        Returns:
            SessionTask or None if not found.
        """
        # Check memory first
        if task_id in self._tasks:
            return self._tasks[task_id]

        # Check database
        task = await self._load_task(task_id)
        if task:
            self._tasks[task_id] = task
        return task

    async def get_task_by_session(self, session_id: str) -> SessionTask | None:
        """Get task by session_id.

        Args:
            session_id: Session identifier.

        Returns:
            SessionTask or None if not found.
        """
        # Check memory first
        for task in self._tasks.values():
            if task.session_id == session_id:
                return task

        # Check database
        task = await self._load_task_by_session(session_id)
        if task:
            self._tasks[task.task_id] = task
        return task

    async def update_task_status(
        self,
        task_id: str,
        status: TaskStatus | str,
        error: str | None = None,
        histories_written: bool | None = None,
    ) -> None:
        """Update task status.

        Args:
            task_id: Task identifier.
            status: New status (TaskStatus enum or string).
            error: Error message if failed.
            histories_written: Whether histories were written.
        """
        task = await self.get_task(task_id)
        if not task:
            logger.warning(f"Task {task_id} not found for status update")
            return

        # Convert string to TaskStatus if needed
        if isinstance(status, str):
            status = TaskStatus(status)

        task.status = status
        task.updated_at = datetime.now()
        if error:
            task.error_message = error
        if histories_written is not None:
            task.histories_written = histories_written

        await self._save_task(task)
        logger.debug(f"Task {task_id} status updated to {status}")

    async def recover_tasks(self) -> list[SessionTask]:
        """Recover pending/running tasks after restart.

        Returns:
            List of recovered tasks that need processing.
        """
        recovered = []
        async for task in self._list_tasks_by_status(TaskStatus.PENDING, TaskStatus.RUNNING):
            # Reset running tasks to pending
            if task.status == TaskStatus.RUNNING:
                task.status = TaskStatus.PENDING
                await self._save_task(task)

            self._tasks[task.task_id] = task
            recovered.append(task)
            logger.info(f"Recovered task {task.task_id} for session {task.session_id}")

        return recovered

    async def _worker(self) -> None:
        """Background worker that processes tasks sequentially."""
        logger.info("Task worker started")

        while True:
            try:
                # Wait for task
                task = await self._queue.get()
                logger.info(f"Processing task {task.task_id}")

                # Get messages
                messages = self._pending_messages.pop(task.session_id, None)
                if messages is None:
                    # Try to load from histories for retry
                    messages = await self._load_messages_from_histories(task.session_id)

                if not messages:
                    logger.error(f"No messages found for session {task.session_id}")
                    await self.update_task_status(
                        task.task_id,
                        TaskStatus.FAILED,
                        error="No messages available"
                    )
                    continue

                # Update status to running
                await self.update_task_status(task.task_id, TaskStatus.RUNNING)

                # Process using workflow if available, otherwise use processor
                try:
                    if self._workflow:
                        result = await self._workflow.run(task.session_id, messages)
                        if result.get("error"):
                            await self.update_task_status(
                                task.task_id,
                                TaskStatus.FAILED,
                                error=result["error"]
                            )
                        else:
                            await self.update_task_status(task.task_id, TaskStatus.COMPLETED)
                            logger.info(f"Task {task.task_id} completed via workflow")
                    elif self._processor:
                        await self._processor(task, messages)
                        await self.update_task_status(task.task_id, TaskStatus.COMPLETED)
                        logger.info(f"Task {task.task_id} completed")
                    else:
                        logger.error("No workflow or processor set for task manager")
                        await self.update_task_status(
                            task.task_id,
                            TaskStatus.FAILED,
                            error="No workflow or processor configured"
                        )
                except Exception as e:
                    logger.exception(f"Task {task.task_id} failed: {e}")
                    await self.update_task_status(
                        task.task_id,
                        TaskStatus.FAILED,
                        error=str(e)
                    )

            except asyncio.CancelledError:
                logger.info("Task worker cancelled")
                break
            except Exception as e:
                logger.exception(f"Unexpected error in worker: {e}")

    async def _save_task(self, task: SessionTask) -> None:
        """Save task to database."""
        if not self.metadata_manager._db:
            return

        await self.metadata_manager._db.execute(
            """
            INSERT OR REPLACE INTO session_tasks
            (task_id, session_id, status, created_at, updated_at, error_message, histories_written)
            VALUES (?, ?, ?, ?, ?, ?, ?)
            """,
            (
                task.task_id,
                task.session_id,
                task.status.value,
                task.created_at.isoformat(),
                task.updated_at.isoformat(),
                task.error_message,
                1 if task.histories_written else 0,
            ),
        )
        await self.metadata_manager._db.commit()

    async def _load_task(self, task_id: str) -> SessionTask | None:
        """Load task from database by task_id."""
        if not self.metadata_manager._db:
            return None

        async with self.metadata_manager._db.execute(
            "SELECT * FROM session_tasks WHERE task_id = ?", (task_id,)
        ) as cursor:
            row = await cursor.fetchone()
            if row:
                return SessionTask.from_dict({
                    "task_id": row[0],
                    "session_id": row[1],
                    "status": row[2],
                    "created_at": row[3],
                    "updated_at": row[4],
                    "error_message": row[5],
                    "histories_written": row[6],
                })
            return None

    async def _load_task_by_session(self, session_id: str) -> SessionTask | None:
        """Load task from database by session_id."""
        if not self.metadata_manager._db:
            return None

        async with self.metadata_manager._db.execute(
            "SELECT * FROM session_tasks WHERE session_id = ?", (session_id,)
        ) as cursor:
            row = await cursor.fetchone()
            if row:
                return SessionTask.from_dict({
                    "task_id": row[0],
                    "session_id": row[1],
                    "status": row[2],
                    "created_at": row[3],
                    "updated_at": row[4],
                    "error_message": row[5],
                    "histories_written": row[6],
                })
            return None

    async def _load_pending_tasks(self) -> None:
        """Load pending/running tasks from database into memory."""
        async for task in self._list_tasks_by_status(TaskStatus.PENDING, TaskStatus.RUNNING):
            self._tasks[task.task_id] = task

    async def _list_tasks_by_status(self, *statuses: TaskStatus) -> "AsyncIterator[SessionTask]":
        """List tasks with given statuses.

        Note: This returns an async generator, need to import AsyncIterator.
        """
        if not self.metadata_manager._db:
            return

        placeholders = ",".join("?" * len(statuses))
        async with self.metadata_manager._db.execute(
            f"SELECT * FROM session_tasks WHERE status IN ({placeholders})",
            [s.value for s in statuses]
        ) as cursor:
            async for row in cursor:
                yield SessionTask.from_dict({
                    "task_id": row[0],
                    "session_id": row[1],
                    "status": row[2],
                    "created_at": row[3],
                    "updated_at": row[4],
                    "error_message": row[5],
                    "histories_written": row[6],
                })

    async def _load_messages_from_histories(self, session_id: str) -> list | None:
        """Load messages from histories file for retry.

        Args:
            session_id: Session identifier.

        Returns:
            List of messages or None if not found.
        """
        import json
        histories_path = settings.memory_dir / "histories" / f"{session_id}.json"
        if histories_path.exists():
            try:
                data = json.loads(histories_path.read_text())
                return data.get("messages", [])
            except Exception as e:
                logger.error(f"Failed to load histories for {session_id}: {e}")
        return None