"""Metadata management with SQLite backend."""

import hashlib
import time
from datetime import datetime
from pathlib import Path

import aiosqlite

from memsvc.config import settings
from memsvc.models.file_meta import FileChange, FileMetadata, IndexStatus


def compute_hash(file_path: Path) -> str:
    """Compute SHA256 hash of a file."""
    sha256 = hashlib.sha256()
    with open(file_path, "rb") as f:
        for chunk in iter(lambda: f.read(8192), b""):
            sha256.update(chunk)
    return sha256.hexdigest()


class MetadataManager:
    """Manages file metadata with SQLite storage."""

    CREATE_TABLE_SQL = """
    CREATE TABLE IF NOT EXISTS file_metadata (
        path TEXT PRIMARY KEY,
        content_hash TEXT NOT NULL,
        file_size INTEGER NOT NULL,
        mod_time TEXT NOT NULL,
        index_status TEXT NOT NULL,
        indexed_at TEXT,
        error_message TEXT
    )
    """

    def __init__(self, db_path: Path | None = None, memory_dir: Path | None = None):
        self.db_path = db_path or settings.db_path
        self.memory_dir = memory_dir or settings.memory_dir
        self.watch_dirs = settings.watch_dirs
        self._db: aiosqlite.Connection | None = None
        self._start_time = time.time()

    async def initialize(self) -> None:
        """Initialize database connection and create tables."""
        self.db_path.parent.mkdir(parents=True, exist_ok=True)
        self._db = await aiosqlite.connect(self.db_path)
        await self._db.execute(self.CREATE_TABLE_SQL)
        await self._db.commit()

    async def close(self) -> None:
        """Close database connection."""
        if self._db:
            await self._db.close()
            self._db = None

    async def get_metadata(self, path: str) -> FileMetadata | None:
        """Get metadata for a file by its relative path."""
        async with self._db.execute(
            "SELECT * FROM file_metadata WHERE path = ?", (path,)
        ) as cursor:
            row = await cursor.fetchone()
            if row:
                return FileMetadata.from_dict({
                    "path": row[0],
                    "content_hash": row[1],
                    "file_size": row[2],
                    "mod_time": row[3],
                    "index_status": row[4],
                    "indexed_at": row[5],
                    "error_message": row[6],
                })
            return None

    async def upsert_metadata(self, metadata: FileMetadata) -> None:
        """Insert or update file metadata."""
        await self._db.execute(
            """
            INSERT OR REPLACE INTO file_metadata
            (path, content_hash, file_size, mod_time, index_status, indexed_at, error_message)
            VALUES (?, ?, ?, ?, ?, ?, ?)
            """,
            (
                metadata.path,
                metadata.content_hash,
                metadata.file_size,
                metadata.mod_time.isoformat(),
                metadata.index_status.value,
                metadata.indexed_at.isoformat() if metadata.indexed_at else None,
                metadata.error_message,
            ),
        )
        await self._db.commit()

    async def delete_metadata(self, path: str) -> None:
        """Delete metadata for a file."""
        await self._db.execute("DELETE FROM file_metadata WHERE path = ?", (path,))
        await self._db.commit()

    async def mark_deleted(self, path: str) -> None:
        """Mark a file as deleted in metadata."""
        await self._db.execute(
            "UPDATE file_metadata SET index_status = ? WHERE path = ?",
            (IndexStatus.DELETED.value, path),
        )
        await self._db.commit()

    async def mark_indexed(self, path: str) -> None:
        """Mark a file as successfully indexed."""
        await self._db.execute(
            "UPDATE file_metadata SET index_status = ?, indexed_at = ?, error_message = NULL WHERE path = ?",
            (IndexStatus.INDEXED.value, datetime.now().isoformat(), path),
        )
        await self._db.commit()

    async def mark_failed(self, path: str, error: str) -> None:
        """Mark a file as failed to index with error message."""
        await self._db.execute(
            "UPDATE file_metadata SET index_status = ?, error_message = ? WHERE path = ?",
            (IndexStatus.FAILED.value, error, path),
        )
        await self._db.commit()

    async def list_all(self) -> list[FileMetadata]:
        """List all file metadata."""
        async with self._db.execute(
            "SELECT * FROM file_metadata WHERE index_status != ?",
            (IndexStatus.DELETED.value,),
        ) as cursor:
            rows = await cursor.fetchall()
            return [
                FileMetadata.from_dict({
                    "path": row[0],
                    "content_hash": row[1],
                    "file_size": row[2],
                    "mod_time": row[3],
                    "index_status": row[4],
                    "indexed_at": row[5],
                    "error_message": row[6],
                })
                for row in rows
            ]

    async def list_pending(self) -> list[FileMetadata]:
        """List files pending indexing."""
        async with self._db.execute(
            "SELECT * FROM file_metadata WHERE index_status = ?",
            (IndexStatus.PENDING.value,),
        ) as cursor:
            rows = await cursor.fetchall()
            return [
                FileMetadata.from_dict({
                    "path": row[0],
                    "content_hash": row[1],
                    "file_size": row[2],
                    "mod_time": row[3],
                    "index_status": row[4],
                    "indexed_at": row[5],
                    "error_message": row[6],
                })
                for row in rows
            ]

    async def count_by_status(self) -> dict[str, int]:
        """Count files by index status."""
        result = {status.value: 0 for status in IndexStatus}
        async with self._db.execute(
            "SELECT index_status, COUNT(*) FROM file_metadata GROUP BY index_status"
        ) as cursor:
            async for row in cursor:
                result[row[0]] = row[1]
        return result

    def detect_changes(self) -> list[FileChange]:
        """Synchronously scan directories to detect changes.

        This is a synchronous method to be called during initialization
        or when explicitly triggered.
        """
        changes = []
        now = datetime.now()

        # Get current files in watched directories
        current_files: dict[str, Path] = {}
        for watch_dir in self.watch_dirs:
            dir_path = self.memory_dir / watch_dir
            if not dir_path.exists():
                continue
            for file_path in dir_path.rglob("*"):
                if file_path.is_file() and file_path.suffix in settings.supported_extensions:
                    rel_path = str(file_path.relative_to(self.memory_dir))
                    current_files[rel_path] = file_path

        # We need to compare with stored metadata (sync version)
        # This will be called during init before async is fully set up
        return changes

    async def scan_and_update(self) -> list[FileChange]:
        """Scan watched directories and update metadata.

        Returns list of detected changes.
        """
        changes = []
        now = datetime.now()

        # Get current files in watched directories
        current_files: dict[str, Path] = {}
        for watch_dir in self.watch_dirs:
            dir_path = self.memory_dir / watch_dir
            if not dir_path.exists():
                dir_path.mkdir(parents=True, exist_ok=True)
                continue
            for file_path in dir_path.rglob("*"):
                if file_path.is_file() and file_path.suffix in settings.supported_extensions:
                    if file_path.stat().st_size > settings.max_file_size:
                        continue
                    rel_path = str(file_path.relative_to(self.memory_dir))
                    current_files[rel_path] = file_path

        # Check for new or modified files
        for rel_path, file_path in current_files.items():
            current_hash = compute_hash(file_path)
            file_stat = file_path.stat()
            stored = await self.get_metadata(rel_path)

            if stored is None:
                # New file
                changes.append(FileChange(
                    path=rel_path,
                    change_type="created",
                    new_hash=current_hash,
                    timestamp=now,
                ))
                await self.upsert_metadata(FileMetadata(
                    path=rel_path,
                    content_hash=current_hash,
                    file_size=file_stat.st_size,
                    mod_time=datetime.fromtimestamp(file_stat.st_mtime),
                    index_status=IndexStatus.PENDING,
                ))
            elif stored.content_hash != current_hash:
                # Modified file
                changes.append(FileChange(
                    path=rel_path,
                    change_type="modified",
                    old_hash=stored.content_hash,
                    new_hash=current_hash,
                    timestamp=now,
                ))
                await self.upsert_metadata(FileMetadata(
                    path=rel_path,
                    content_hash=current_hash,
                    file_size=file_stat.st_size,
                    mod_time=datetime.fromtimestamp(file_stat.st_mtime),
                    index_status=IndexStatus.PENDING,
                ))

        # Check for deleted files
        all_stored = await self.list_all()
        for stored in all_stored:
            if stored.path not in current_files and stored.index_status != IndexStatus.DELETED:
                changes.append(FileChange(
                    path=stored.path,
                    change_type="deleted",
                    old_hash=stored.content_hash,
                    timestamp=now,
                ))
                await self.mark_deleted(stored.path)

        return changes

    def get_uptime(self) -> float:
        """Get service uptime in seconds."""
        return time.time() - self._start_time