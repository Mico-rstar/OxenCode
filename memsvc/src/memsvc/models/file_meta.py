"""File metadata models."""

from dataclasses import dataclass, field
from datetime import datetime
from enum import Enum
from typing import Self


class IndexStatus(str, Enum):
    """Index status for a file."""

    PENDING = "pending"
    INDEXED = "indexed"
    FAILED = "failed"
    DELETED = "deleted"


@dataclass
class FileMetadata:
    """Metadata for a memory file."""

    path: str  # Relative path from memory_dir
    content_hash: str  # SHA256 hash
    file_size: int
    mod_time: datetime
    index_status: IndexStatus = IndexStatus.PENDING
    indexed_at: datetime | None = None
    error_message: str | None = None

    def to_dict(self) -> dict:
        """Convert to dictionary for storage."""
        return {
            "path": self.path,
            "content_hash": self.content_hash,
            "file_size": self.file_size,
            "mod_time": self.mod_time.isoformat(),
            "index_status": self.index_status.value,
            "indexed_at": self.indexed_at.isoformat() if self.indexed_at else None,
            "error_message": self.error_message,
        }

    @classmethod
    def from_dict(cls, data: dict) -> Self:
        """Create from dictionary."""
        return cls(
            path=data["path"],
            content_hash=data["content_hash"],
            file_size=data["file_size"],
            mod_time=datetime.fromisoformat(data["mod_time"]),
            index_status=IndexStatus(data["index_status"]),
            indexed_at=(
                datetime.fromisoformat(data["indexed_at"]) if data.get("indexed_at") else None
            ),
            error_message=data.get("error_message"),
        )


@dataclass
class FileChange:
    """Represents a file change detected."""

    path: str  # Relative path
    change_type: str  # "created", "modified", "deleted"
    old_hash: str | None = None
    new_hash: str | None = None
    timestamp: datetime = field(default_factory=datetime.now)

    def to_dict(self) -> dict:
        """Convert to dictionary."""
        return {
            "path": self.path,
            "change_type": self.change_type,
            "old_hash": self.old_hash,
            "new_hash": self.new_hash,
            "timestamp": self.timestamp.isoformat(),
        }


@dataclass
class ServiceStatus:
    """Service status information."""

    watch_enabled: bool
    watch_active: bool
    total_files: int
    pending_count: int
    indexed_count: int
    failed_count: int
    memory_dir: str
    uptime_seconds: float

    def to_dict(self) -> dict:
        """Convert to dictionary."""
        return {
            "watch_enabled": self.watch_enabled,
            "watch_active": self.watch_active,
            "total_files": self.total_files,
            "pending_count": self.pending_count,
            "indexed_count": self.indexed_count,
            "failed_count": self.failed_count,
            "memory_dir": self.memory_dir,
            "uptime_seconds": self.uptime_seconds,
        }