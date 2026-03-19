"""Tests for session API endpoints."""

import json
import tempfile
from pathlib import Path

import pytest
from fastapi.testclient import TestClient

from memsvc.api.router import (
    set_metadata_manager,
    set_file_watcher,
    set_memory_indexer,
    set_task_manager,
    set_session_compressor,
)
from memsvc.config import settings
from memsvc.core.metadata import MetadataManager
from memsvc.core.task_manager import TaskManager
from memsvc.core.compressor import SessionCompressor
from memsvc.core.llm import MockLLM


@pytest.fixture
def setup_services(tmp_path: Path):
    """Set up services for testing."""
    import asyncio

    db_path = tmp_path / "metadata.db"
    memory_dir = tmp_path / "memory"
    memory_dir.mkdir(parents=True, exist_ok=True)

    # Create memory subdirectories
    for subdir in ["experience", "knowledge", "notes", "histories", "inner"]:
        (memory_dir / subdir).mkdir(exist_ok=True)

    async def init_services():
        # Initialize metadata manager
        metadata_manager = MetadataManager(db_path=db_path, memory_dir=memory_dir)
        await metadata_manager.initialize()
        set_metadata_manager(metadata_manager)

        # Initialize task manager
        task_manager = TaskManager(metadata_manager=metadata_manager)
        await task_manager.initialize()

        # Use mock LLM for testing
        llm = MockLLM()

        # Initialize compressor
        compressor = SessionCompressor(llm=llm, memory_dir=memory_dir)
        set_session_compressor(compressor)

        # Set up processor
        async def process_task(task, messages):
            await compressor.process_session(
                task,
                messages,
                skip_histories=task.histories_written,
            )
            await task_manager.update_task_status(task.task_id, "completed")

        task_manager.set_processor(process_task)
        set_task_manager(task_manager)

        return task_manager, compressor, memory_dir, metadata_manager

    task_manager, compressor, memory_dir, metadata_manager = asyncio.get_event_loop().run_until_complete(init_services())

    yield task_manager, compressor, memory_dir

    # Cleanup
    async def cleanup():
        await task_manager.close()
        await metadata_manager.close()
        set_metadata_manager(None)
        set_task_manager(None)
        set_session_compressor(None)

    asyncio.get_event_loop().run_until_complete(cleanup())


@pytest.fixture
def client():
    """Create test client."""
    from memsvc.main import app
    return TestClient(app)


class TestSessionAPI:
    """Tests for session API endpoints."""

    def test_commit_session(self, setup_services, client):
        """Test committing a session."""
        task_manager, compressor, memory_dir = setup_services

        response = client.post(
            "/api/v1/commit_session",
            json={
                "session_id": "test-session-1",
                "messages": [
                    {"role": "user", "content": "Hello"},
                    {"role": "assistant", "content": "Hi there!"},
                ],
            },
        )

        assert response.status_code == 200
        data = response.json()
        assert "task_id" in data
        assert data["task_id"].startswith("task_")

    def test_commit_session_duplicate(self, setup_services, client):
        """Test that duplicate pending sessions are rejected."""
        task_manager, compressor, memory_dir = setup_services

        # First request
        response = client.post(
            "/api/v1/commit_session",
            json={
                "session_id": "test-session-dup",
                "messages": [{"role": "user", "content": "Test"}],
            },
        )
        assert response.status_code == 200

        # Second request should fail
        response = client.post(
            "/api/v1/commit_session",
            json={
                "session_id": "test-session-dup",
                "messages": [{"role": "user", "content": "Test"}],
            },
        )
        assert response.status_code == 409

    def test_get_task_status(self, setup_services, client):
        """Test getting task status."""
        task_manager, compressor, memory_dir = setup_services

        # Create a task
        response = client.post(
            "/api/v1/commit_session",
            json={
                "session_id": "test-session-status",
                "messages": [{"role": "user", "content": "Test"}],
            },
        )
        task_id = response.json()["task_id"]

        # Get status
        response = client.get(f"/api/v1/task/{task_id}/status")
        assert response.status_code == 200
        data = response.json()
        assert data["task_id"] == task_id
        assert data["session_id"] == "test-session-status"
        assert data["status"] in ("pending", "running", "completed")

    def test_get_task_status_not_found(self, setup_services, client):
        """Test getting status of non-existent task."""
        response = client.get("/api/v1/task/nonexistent_task/status")
        assert response.status_code == 404

    def test_get_notes(self, setup_services, client):
        """Test getting notes for a session."""
        task_manager, compressor, memory_dir = setup_services

        # Manually create a notes file
        notes_path = memory_dir / "notes" / "test-session-notes.md"
        notes_content = """---
description: Test session notes
created_at: 2024-01-01T12:00:00
---

## Test Notes
This is a test.
"""
        notes_path.write_text(notes_content)

        response = client.get("/api/v1/notes/test-session-notes")
        assert response.status_code == 200
        data = response.json()
        assert data["exists"] is True
        assert data["content"] == notes_content

    def test_get_notes_not_found(self, setup_services, client):
        """Test getting notes that don't exist."""
        response = client.get("/api/v1/notes/nonexistent-session")
        assert response.status_code == 200
        data = response.json()
        assert data["exists"] is False
        assert data["content"] is None

    def test_retry_session(self, setup_services, client):
        """Test retrying a session."""
        import asyncio
        task_manager, compressor, memory_dir = setup_services

        # Create a histories file
        histories_path = memory_dir / "histories" / "test-session-retry.json"
        histories_data = {
            "session_id": "test-session-retry",
            "messages": [
                {"role": "user", "content": "Test"},
            ],
            "created_at": "2024-01-01T12:00:00",
        }
        histories_path.write_text(json.dumps(histories_data))

        # Create a failed task
        async def setup_failed_task():
            task_id = await task_manager.create_task(
                "test-session-retry",
                histories_data["messages"],
            )
            await task_manager.update_task_status(
                task_id,
                "failed",
                error="Previous error",
            )
            return task_id

        old_task_id = asyncio.get_event_loop().run_until_complete(setup_failed_task())

        # Wait a bit
        asyncio.get_event_loop().run_until_complete(asyncio.sleep(0.1))

        # Retry
        response = client.post(
            "/api/v1/retry_session",
            json={"session_id": "test-session-retry"},
        )
        assert response.status_code == 200
        new_task_id = response.json()["task_id"]

    def test_retry_session_no_histories(self, setup_services, client):
        """Test retrying a session without histories."""
        response = client.post(
            "/api/v1/retry_session",
            json={"session_id": "no-histories-session"},
        )
        assert response.status_code == 404