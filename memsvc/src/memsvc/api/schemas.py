"""Pydantic schemas for API."""

from datetime import datetime
from enum import Enum
from typing import Any

from pydantic import BaseModel, Field


class IndexStatus(str, Enum):
    """Index status for a file."""

    PENDING = "pending"
    INDEXED = "indexed"
    FAILED = "failed"
    DELETED = "deleted"


class FileMetadataResponse(BaseModel):
    """Response model for file metadata."""

    path: str
    content_hash: str
    file_size: int
    mod_time: datetime
    index_status: IndexStatus
    indexed_at: datetime | None = None
    error_message: str | None = None


class FileChangeResponse(BaseModel):
    """Response model for file change."""

    path: str
    change_type: str
    old_hash: str | None = None
    new_hash: str | None = None
    timestamp: datetime


class ServiceStatusResponse(BaseModel):
    """Response model for service status."""

    watch_enabled: bool
    watch_active: bool
    total_files: int
    pending_count: int
    indexed_count: int
    failed_count: int
    memory_dir: str
    uptime_seconds: float


class HealthResponse(BaseModel):
    """Response model for health check."""

    status: str = "ok"
    version: str = "0.1.0"


class ScanResponse(BaseModel):
    """Response model for scan endpoint."""

    changes: list[FileChangeResponse]
    total_changes: int


class MetadataListResponse(BaseModel):
    """Response model for metadata list."""

    files: list[FileMetadataResponse]
    total: int


class ErrorResponse(BaseModel):
    """Error response model."""

    error: str
    detail: str | None = None