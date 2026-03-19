"""Tests for API endpoints."""

import tempfile
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient
from httpx import AsyncClient

from memsvc.api.router import router, set_metadata_manager, set_file_watcher
from memsvc.core.metadata import MetadataManager
from memsvc.models.file_meta import FileMetadata, IndexStatus


@pytest.fixture
def client(tmp_path: Path):
    """Create a test client with mocked dependencies."""
    # Create mock metadata manager
    mock_manager = AsyncMock(spec=MetadataManager)
    mock_manager.get_uptime.return_value = 100.0

    # Set mock dependencies before creating test client
    set_metadata_manager(mock_manager)
    set_file_watcher(None)

    # Create test app without lifespan to avoid real initialization
    test_app = FastAPI(lifespan=None)
    test_app.include_router(router, prefix="/api/v1")

    with TestClient(test_app) as test_client:
        yield test_client, mock_manager

    # Cleanup
    set_metadata_manager(None)
    set_file_watcher(None)


class TestHealthEndpoint:
    """Tests for /health endpoint."""

    def test_health_check(self, client):
        """Test health check returns ok."""
        test_client, _ = client
        response = test_client.get("/api/v1/health")
        assert response.status_code == 200
        assert response.json()["status"] == "ok"


class TestStatusEndpoint:
    """Tests for /status endpoint."""

    def test_get_status(self, client):
        """Test getting service status."""
        test_client, mock_manager = client
        mock_manager.count_by_status.return_value = {
            "pending": 5,
            "indexed": 10,
            "failed": 1,
            "deleted": 0,
        }

        response = test_client.get("/api/v1/status")

        assert response.status_code == 200
        data = response.json()
        assert data["watch_enabled"] is True
        assert data["watch_active"] is False
        assert data["total_files"] == 16
        assert data["pending_count"] == 5
        assert data["indexed_count"] == 10


class TestMetadataEndpoints:
    """Tests for /metadata endpoints."""

    def test_list_metadata(self, client):
        """Test listing all metadata."""
        test_client, mock_manager = client
        mock_manager.list_all.return_value = [
            FileMetadata(
                path="experience/test.md",
                content_hash="hash1",
                file_size=100,
                mod_time="2024-01-01T12:00:00",
                index_status=IndexStatus.PENDING,
            ),
        ]

        response = test_client.get("/api/v1/metadata")

        assert response.status_code == 200
        data = response.json()
        print(data)
        assert data["total"] == 1
        assert len(data["files"]) == 1

    def test_get_metadata(self, client):
        """Test getting specific metadata."""
        test_client, mock_manager = client
        mock_manager.get_metadata.return_value = FileMetadata(
            path="experience/test.md",
            content_hash="hash1",
            file_size=100,
            mod_time="2024-01-01T12:00:00",
            index_status=IndexStatus.PENDING,
        )

        response = test_client.get("/api/v1/metadata/experience/test.md")

        assert response.status_code == 200
        data = response.json()
        assert data["path"] == "experience/test.md"

    def test_get_metadata_not_found(self, client):
        """Test getting nonexistent metadata."""
        test_client, mock_manager = client
        mock_manager.get_metadata.return_value = None

        response = test_client.get("/api/v1/metadata/nonexistent.md")

        assert response.status_code == 404


class TestScanEndpoint:
    """Tests for /scan endpoint."""

    def test_scan_files(self, client):
        """Test manual scan."""
        test_client, mock_manager = client
        mock_manager.scan_and_update.return_value = []

        response = test_client.post("/api/v1/scan")

        assert response.status_code == 200
        data = response.json()
        assert data["total_changes"] == 0