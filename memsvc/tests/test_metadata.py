"""Tests for metadata management."""

import hashlib
import tempfile
from datetime import datetime
from pathlib import Path

import pytest

from memsvc.core.metadata import MetadataManager, compute_hash
from memsvc.models.file_meta import FileMetadata, IndexStatus


class TestComputeHash:
    """Tests for hash computation."""

    def test_compute_hash_empty_file(self, tmp_path: Path):
        """Test hash of empty file."""
        file_path = tmp_path / "empty.txt"
        file_path.write_text("")

        expected = hashlib.sha256(b"").hexdigest()
        assert compute_hash(file_path) == expected

    def test_compute_hash_simple_content(self, tmp_path: Path):
        """Test hash of simple content."""
        content = "Hello, World!"
        file_path = tmp_path / "test.txt"
        file_path.write_text(content)

        expected = hashlib.sha256(content.encode()).hexdigest()
        assert compute_hash(file_path) == expected

    def test_compute_hash_large_file(self, tmp_path: Path):
        """Test hash of larger file (tests chunked reading)."""
        content = "x" * 10000  # Larger than chunk size
        file_path = tmp_path / "large.txt"
        file_path.write_text(content)

        expected = hashlib.sha256(content.encode()).hexdigest()
        assert compute_hash(file_path) == expected


class TestFileMetadata:
    """Tests for FileMetadata model."""

    def test_to_dict_and_from_dict(self):
        """Test serialization round-trip."""
        metadata = FileMetadata(
            path="experience/test.md",
            content_hash="abc123",
            file_size=100,
            mod_time=datetime(2024, 1, 1, 12, 0, 0),
            index_status=IndexStatus.PENDING,
            indexed_at=datetime(2024, 1, 1, 13, 0, 0),
            error_message=None,
        )

        data = metadata.to_dict()
        restored = FileMetadata.from_dict(data)

        assert restored.path == metadata.path
        assert restored.content_hash == metadata.content_hash
        assert restored.file_size == metadata.file_size
        assert restored.mod_time == metadata.mod_time
        assert restored.index_status == metadata.index_status
        assert restored.indexed_at == metadata.indexed_at

    def test_to_dict_without_indexed_at(self):
        """Test serialization without indexed_at."""
        metadata = FileMetadata(
            path="experience/test.md",
            content_hash="abc123",
            file_size=100,
            mod_time=datetime(2024, 1, 1, 12, 0, 0),
            index_status=IndexStatus.PENDING,
        )

        data = metadata.to_dict()
        assert data["indexed_at"] is None

        restored = FileMetadata.from_dict(data)
        assert restored.indexed_at is None


class TestMetadataManager:
    """Tests for MetadataManager."""

    @pytest.fixture
    async def manager(self, tmp_path: Path):
        """Create a metadata manager with temp paths."""
        db_path = tmp_path / "metadata.db"
        memory_dir = tmp_path / "memory"
        memory_dir.mkdir(parents=True, exist_ok=True)

        manager = MetadataManager(db_path=db_path, memory_dir=memory_dir)
        await manager.initialize()
        yield manager
        await manager.close()

    @pytest.mark.asyncio
    async def test_upsert_and_get(self, manager: MetadataManager):
        """Test upsert and get operations."""
        metadata = FileMetadata(
            path="experience/test.md",
            content_hash="abc123",
            file_size=100,
            mod_time=datetime(2024, 1, 1, 12, 0, 0),
            index_status=IndexStatus.PENDING,
        )

        await manager.upsert_metadata(metadata)

        result = await manager.get_metadata("experience/test.md")
        assert result is not None
        assert result.path == metadata.path
        assert result.content_hash == metadata.content_hash

    @pytest.mark.asyncio
    async def test_get_nonexistent(self, manager: MetadataManager):
        """Test getting nonexistent metadata."""
        result = await manager.get_metadata("nonexistent.md")
        assert result is None

    @pytest.mark.asyncio
    async def test_delete(self, manager: MetadataManager):
        """Test delete operation."""
        metadata = FileMetadata(
            path="experience/test.md",
            content_hash="abc123",
            file_size=100,
            mod_time=datetime(2024, 1, 1, 12, 0, 0),
            index_status=IndexStatus.PENDING,
        )

        await manager.upsert_metadata(metadata)
        await manager.delete_metadata("experience/test.md")

        result = await manager.get_metadata("experience/test.md")
        assert result is None

    @pytest.mark.asyncio
    async def test_mark_deleted(self, manager: MetadataManager):
        """Test mark as deleted operation."""
        metadata = FileMetadata(
            path="experience/test.md",
            content_hash="abc123",
            file_size=100,
            mod_time=datetime(2024, 1, 1, 12, 0, 0),
            index_status=IndexStatus.PENDING,
        )

        await manager.upsert_metadata(metadata)
        await manager.mark_deleted("experience/test.md")

        # Should still exist but with DELETED status
        result = await manager.get_metadata("experience/test.md")
        assert result is not None
        assert result.index_status == IndexStatus.DELETED

    @pytest.mark.asyncio
    async def test_list_all(self, manager: MetadataManager):
        """Test list all operation."""
        for i in range(3):
            metadata = FileMetadata(
                path=f"experience/test{i}.md",
                content_hash=f"hash{i}",
                file_size=100,
                mod_time=datetime(2024, 1, 1, 12, 0, 0),
                index_status=IndexStatus.PENDING,
            )
            await manager.upsert_metadata(metadata)

        all_files = await manager.list_all()
        assert len(all_files) == 3

    @pytest.mark.asyncio
    async def test_list_pending(self, manager: MetadataManager):
        """Test list pending operation."""
        # Create files with different statuses
        for i, status in enumerate([IndexStatus.PENDING, IndexStatus.INDEXED, IndexStatus.PENDING]):
            metadata = FileMetadata(
                path=f"experience/test{i}.md",
                content_hash=f"hash{i}",
                file_size=100,
                mod_time=datetime(2024, 1, 1, 12, 0, 0),
                index_status=status,
            )
            await manager.upsert_metadata(metadata)

        pending = await manager.list_pending()
        assert len(pending) == 2

    @pytest.mark.asyncio
    async def test_count_by_status(self, manager: MetadataManager):
        """Test count by status operation."""
        statuses = [IndexStatus.PENDING, IndexStatus.INDEXED, IndexStatus.FAILED, IndexStatus.PENDING]
        for i, status in enumerate(statuses):
            metadata = FileMetadata(
                path=f"experience/test{i}.md",
                content_hash=f"hash{i}",
                file_size=100,
                mod_time=datetime(2024, 1, 1, 12, 0, 0),
                index_status=status,
            )
            await manager.upsert_metadata(metadata)

        counts = await manager.count_by_status()
        assert counts[IndexStatus.PENDING.value] == 2
        assert counts[IndexStatus.INDEXED.value] == 1
        assert counts[IndexStatus.FAILED.value] == 1


class TestScanAndUpdate:
    """Tests for scan_and_update functionality."""

    @pytest.fixture
    async def manager(self, tmp_path: Path):
        """Create a metadata manager with temp paths and directories."""
        db_path = tmp_path / "metadata.db"
        memory_dir = tmp_path / "memory"

        manager = MetadataManager(db_path=db_path, memory_dir=memory_dir)
        await manager.initialize()
        yield manager
        await manager.close()

    @pytest.mark.asyncio
    async def test_detect_new_file(self, manager: MetadataManager, tmp_path: Path):
        """Test detecting a new file."""
        memory_dir = tmp_path / "memory"
        exp_dir = memory_dir / "experience"
        exp_dir.mkdir(parents=True, exist_ok=True)

        # Create a new file
        test_file = exp_dir / "test.md"
        test_file.write_text("# Test content")

        changes = await manager.scan_and_update()

        assert len(changes) == 1
        assert changes[0].change_type == "created"
        assert changes[0].path == "experience/test.md"

    @pytest.mark.asyncio
    async def test_detect_modified_file(self, manager: MetadataManager, tmp_path: Path):
        """Test detecting a modified file."""
        memory_dir = tmp_path / "memory"
        exp_dir = memory_dir / "experience"
        exp_dir.mkdir(parents=True, exist_ok=True)

        # Create a file and add to metadata
        test_file = exp_dir / "test.md"
        test_file.write_text("Original content")

        # First scan to register the file
        await manager.scan_and_update()

        # Modify the file
        test_file.write_text("Modified content")

        # Second scan should detect modification
        changes = await manager.scan_and_update()

        assert len(changes) == 1
        assert changes[0].change_type == "modified"

    @pytest.mark.asyncio
    async def test_detect_deleted_file(self, manager: MetadataManager, tmp_path: Path):
        """Test detecting a deleted file."""
        memory_dir = tmp_path / "memory"
        exp_dir = memory_dir / "experience"
        exp_dir.mkdir(parents=True, exist_ok=True)

        # Create a file and add to metadata
        test_file = exp_dir / "test.md"
        test_file.write_text("Content")

        # First scan to register the file
        await manager.scan_and_update()

        # Delete the file
        test_file.unlink()

        # Second scan should detect deletion
        changes = await manager.scan_and_update()

        assert len(changes) == 1
        assert changes[0].change_type == "deleted"

    @pytest.mark.asyncio
    async def test_ignore_unsupported_extension(self, manager: MetadataManager, tmp_path: Path):
        """Test ignoring files with unsupported extensions."""
        memory_dir = tmp_path / "memory"
        exp_dir = memory_dir / "experience"
        exp_dir.mkdir(parents=True, exist_ok=True)

        # Create a file with unsupported extension
        test_file = exp_dir / "test.pdf"
        test_file.write_text("PDF content")

        changes = await manager.scan_and_update()

        assert len(changes) == 0