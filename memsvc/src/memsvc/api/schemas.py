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
    trigger_docs: int = 0
    search_docs: int = 0


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


# === RAG API Schemas ===


class ReEmbedRequest(BaseModel):
    """Request model for re_embed endpoint."""

    types: list[str] | None = None  # Filter by memory types (experience, knowledge, notes)


class ReEmbedResponse(BaseModel):
    """Response model for re_embed endpoint."""

    updated_files: list[str]
    indexed_count: int
    skipped_count: int
    errors: list[str] | None = None


class SearchMemoryRequest(BaseModel):
    """Request model for search_memory endpoint."""

    queries: list[str]
    top_k: int = 5
    types: list[str] | None = None  # Filter by memory types


class MemoryResult(BaseModel):
    """Single memory search result."""

    id: str
    description: str
    score: float
    excerpt: str  # Matching chunk or truncated content


class SearchMemoryResponse(BaseModel):
    """Response model for search_memory endpoint."""

    results: list[MemoryResult]


class TriggerMemoryRequest(BaseModel):
    """Request model for trigger_memory endpoint."""

    query: str
    threshold: float = 0.7


class TriggerMemoryResponse(BaseModel):
    """Response model for trigger_memory endpoint."""

    has_relevant: bool
    hint: str | None = None  # Brief description if matched
    score: float = 0.0


class LoadMemoryRequest(BaseModel):
    """Request model for load_memory endpoint."""

    ids: list[str]


class MemoryContent(BaseModel):
    """Full memory content for load response."""

    id: str
    content: str
    source: str
    description: str | None = None


class LoadMemoryResponse(BaseModel):
    """Response model for load_memory endpoint."""

    memories: list[MemoryContent]


# === Session API Schemas ===


class MessageSchema(BaseModel):
    """A single message in a session."""

    role: str
    content: str
    timestamp: datetime | None = None


class CommitSessionRequest(BaseModel):
    """Request model for commit_session endpoint."""

    session_id: str
    messages: list[MessageSchema]


class CommitSessionResponse(BaseModel):
    """Response model for commit_session endpoint."""

    task_id: str


class TaskStatusResponse(BaseModel):
    """Response model for task status endpoint."""

    task_id: str
    session_id: str
    status: str
    created_at: datetime
    updated_at: datetime
    error_message: str | None = None
    histories_written: bool = False


class NotesResponse(BaseModel):
    """Response model for notes endpoint."""

    session_id: str
    content: str | None = None
    exists: bool


class RetrySessionRequest(BaseModel):
    """Request model for retry_session endpoint."""

    session_id: str