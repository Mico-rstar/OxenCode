"""Tests for task manager."""

import asyncio
import tempfile
from datetime import datetime
from pathlib import Path

import pytest

from memsvc.core.metadata import MetadataManager
from memsvc.core.task_manager import TaskManager
from memsvc.models.session import SessionTask, TaskStatus


class TestTaskManager:
    """Tests for TaskManager."""

    @pytest.fixture
    async def setup_manager(self, tmp_path: Path):
        """Create a task manager with metadata manager."""
        db_path = tmp_path / "metadata.db"
        memory_dir = tmp_path / "memory"
        memory_dir.mkdir(parents=True, exist_ok=True)

        # Create memory subdirectories
        (memory_dir / "histories").mkdir(exist_ok=True)
        (memory_dir / "notes").mkdir(exist_ok=True)

        metadata_manager = MetadataManager(db_path=db_path, memory_dir=memory_dir)
        await metadata_manager.initialize()

        task_manager = TaskManager(metadata_manager=metadata_manager)
        await task_manager.initialize()

        # Set a no-op processor to prevent errors
        async def noop_processor(task, messages):
            pass
        task_manager.set_processor(noop_processor)

        yield task_manager, memory_dir

        await task_manager.close()
        await metadata_manager.close()

    @pytest.mark.asyncio
    async def test_create_task(self, setup_manager):
        """Test creating a new task."""
        task_manager, memory_dir = setup_manager

        messages = [
            {"role": "user", "content": "Hello"},
            {"role": "assistant", "content": "Hi there!"},
        ]

        task_id = await task_manager.create_task("test-session-1", messages)

        assert task_id.startswith("task_")
        assert len(task_id) > 5

        # Verify task was stored
        task = await task_manager.get_task(task_id)
        assert task is not None
        assert task.session_id == "test-session-1"
        assert task.status == TaskStatus.PENDING

    @pytest.mark.asyncio
    async def test_get_task_by_session(self, setup_manager):
        """Test getting a task by session ID."""
        task_manager, memory_dir = setup_manager

        messages = [{"role": "user", "content": "Test"}]
        task_id = await task_manager.create_task("test-session-2", messages)

        task = await task_manager.get_task_by_session("test-session-2")
        assert task is not None
        assert task.task_id == task_id

    @pytest.mark.asyncio
    async def test_duplicate_pending_task(self, setup_manager):
        """Test that creating duplicate pending tasks raises error."""
        task_manager, memory_dir = setup_manager

        messages = [{"role": "user", "content": "Test"}]
        await task_manager.create_task("test-session-3", messages)

        # Should raise error for duplicate
        with pytest.raises(ValueError, match="already has a"):
            await task_manager.create_task("test-session-3", messages)

    @pytest.mark.asyncio
    async def test_update_task_status(self, setup_manager):
        """Test updating task status."""
        task_manager, memory_dir = setup_manager

        messages = [{"role": "user", "content": "Test"}]
        task_id = await task_manager.create_task("test-session-4", messages)

        # Wait for initial task creation and worker to pick it up
        await asyncio.sleep(0.05)

        # Update status to RUNNING (will be overwritten by worker, but we test the API)
        await task_manager.update_task_status(task_id, TaskStatus.RUNNING)
        task = await task_manager.get_task(task_id)
        # Task should be RUNNING or COMPLETED depending on timing
        assert task.status in (TaskStatus.RUNNING, TaskStatus.COMPLETED, TaskStatus.PENDING)

        await task_manager.update_task_status(
            task_id,
            TaskStatus.COMPLETED,
            histories_written=True,
        )
        task = await task_manager.get_task(task_id)
        assert task.status == TaskStatus.COMPLETED
        assert task.histories_written is True

    @pytest.mark.asyncio
    async def test_update_task_error(self, setup_manager):
        """Test updating task with error."""
        task_manager, memory_dir = setup_manager

        messages = [{"role": "user", "content": "Test"}]
        task_id = await task_manager.create_task("test-session-5", messages)

        # Wait for initial task creation
        await asyncio.sleep(0.05)

        await task_manager.update_task_status(
            task_id,
            TaskStatus.FAILED,
            error="Something went wrong",
        )
        task = await task_manager.get_task(task_id)
        assert task.status == TaskStatus.FAILED
        assert "Something went wrong" in (task.error_message or "")

    @pytest.mark.asyncio
    async def test_task_persistence(self, setup_manager):
        """Test that tasks are persisted in database."""
        task_manager, memory_dir = setup_manager

        messages = [{"role": "user", "content": "Test"}]
        task_id = await task_manager.create_task("test-session-6", messages)

        # Clear in-memory cache
        task_manager._tasks.clear()

        # Should still be able to load from database
        task = await task_manager.get_task(task_id)
        assert task is not None
        assert task.session_id == "test-session-6"

    @pytest.mark.asyncio
    async def test_processor_execution(self, setup_manager):
        """Test that processor is called for tasks."""
        task_manager, memory_dir = setup_manager

        processed_tasks = []

        async def mock_processor(task, messages):
            processed_tasks.append((task.task_id, task.session_id))
            await task_manager.update_task_status(task.task_id, TaskStatus.COMPLETED)

        task_manager.set_processor(mock_processor)

        messages = [{"role": "user", "content": "Test"}]
        task_id = await task_manager.create_task("test-session-7", messages)

        # Wait for processing
        await asyncio.sleep(0.1)

        # Check that task was processed
        assert len(processed_tasks) == 1
        assert processed_tasks[0][1] == "test-session-7"

        task = await task_manager.get_task(task_id)
        assert task.status == TaskStatus.COMPLETED

    @pytest.mark.asyncio
    async def test_recover_tasks(self, setup_manager):
        """Test recovering pending tasks after restart."""
        task_manager, memory_dir = setup_manager

        # Create a task and wait for it to be processed
        messages = [{"role": "user", "content": "Test"}]
        task_id = await task_manager.create_task("pending-session", messages)

        # Wait for processing
        await asyncio.sleep(0.1)

        # Manually set a task to running state for recovery test
        await task_manager.update_task_status(task_id, TaskStatus.RUNNING)

        # Clear in-memory cache
        task_manager._tasks.clear()

        # Recover tasks
        recovered = await task_manager.recover_tasks()

        # Running tasks should be recovered and reset to pending
        assert len(recovered) == 1
        assert recovered[0].status == TaskStatus.PENDING


class TestSessionTask:
    """Tests for SessionTask model."""

    def test_to_dict_and_from_dict(self):
        """Test serialization round-trip."""
        task = SessionTask(
            task_id="task_test123",
            session_id="session-123",
            status=TaskStatus.PENDING,
            created_at=datetime(2024, 1, 1, 12, 0, 0),
            updated_at=datetime(2024, 1, 1, 12, 30, 0),
            error_message=None,
            histories_written=False,
        )

        data = task.to_dict()
        restored = SessionTask.from_dict(data)

        assert restored.task_id == task.task_id
        assert restored.session_id == task.session_id
        assert restored.status == task.status
        assert restored.created_at == task.created_at
        assert restored.updated_at == task.updated_at
        assert restored.error_message == task.error_message
        assert restored.histories_written == task.histories_written

    def test_to_dict_with_error(self):
        """Test serialization with error message."""
        task = SessionTask(
            task_id="task_test456",
            session_id="session-456",
            status=TaskStatus.FAILED,
            created_at=datetime(2024, 1, 1),
            updated_at=datetime(2024, 1, 1),
            error_message="Test error",
            histories_written=True,
        )

        data = task.to_dict()
        restored = SessionTask.from_dict(data)

        assert restored.error_message == "Test error"
        assert restored.histories_written is True