"""API router for memory service."""

from fastapi import APIRouter, HTTPException, Depends

from memsvc.api.schemas import (
    ErrorResponse,
    FileChangeResponse,
    HealthResponse,
    MetadataListResponse,
    FileMetadataResponse,
    ScanResponse,
    ServiceStatusResponse,
    IndexStatus,
)
from memsvc.config import settings
from memsvc.core.metadata import MetadataManager
from memsvc.core.watcher import FileWatcher

router = APIRouter()

# Global instances (set by app lifespan or tests)
_metadata_manager: MetadataManager | None = None
_file_watcher: FileWatcher | None = None


def set_metadata_manager(manager: MetadataManager | None) -> None:
    """Set metadata manager instance (used by app lifespan or tests)."""
    global _metadata_manager
    _metadata_manager = manager


def set_file_watcher(watcher: FileWatcher | None) -> None:
    """Set file watcher instance (used by app lifespan or tests)."""
    global _file_watcher
    _file_watcher = watcher


def get_metadata_manager() -> MetadataManager:
    """Dependency to get metadata manager instance."""
    if _metadata_manager is None:
        raise HTTPException(status_code=503, detail="Service not initialized")
    return _metadata_manager


def get_file_watcher() -> FileWatcher | None:
    """Dependency to get file watcher instance."""
    return _file_watcher


@router.get("/health", response_model=HealthResponse)
async def health_check() -> HealthResponse:
    """Health check endpoint."""
    return HealthResponse()


@router.get("/status", response_model=ServiceStatusResponse)
async def get_status(
    manager: MetadataManager = Depends(get_metadata_manager),
    watcher: FileWatcher | None = Depends(get_file_watcher),
) -> ServiceStatusResponse:
    """Get service status."""
    counts = await manager.count_by_status()
    total = sum(counts.values())

    return ServiceStatusResponse(
        watch_enabled=settings.watch_enabled,
        watch_active=watcher.is_active if watcher else False,
        total_files=total,
        pending_count=counts.get(IndexStatus.PENDING.value, 0),
        indexed_count=counts.get(IndexStatus.INDEXED.value, 0),
        failed_count=counts.get(IndexStatus.FAILED.value, 0),
        memory_dir=str(settings.memory_dir),
        uptime_seconds=manager.get_uptime(),
    )


@router.get("/metadata", response_model=MetadataListResponse)
async def list_metadata(
    manager: MetadataManager = Depends(get_metadata_manager),
) -> MetadataListResponse:
    """List all file metadata."""
    files = await manager.list_all()

    return MetadataListResponse(
        files=[
            FileMetadataResponse(
                path=f.path,
                content_hash=f.content_hash,
                file_size=f.file_size,
                mod_time=f.mod_time,
                index_status=IndexStatus(f.index_status.value),
                indexed_at=f.indexed_at,
                error_message=f.error_message,
            )
            for f in files
        ],
        total=len(files),
    )


@router.get(
    "/metadata/{path:path}",
    response_model=FileMetadataResponse,
    responses={404: {"model": ErrorResponse}},
)
async def get_metadata(
    path: str,
    manager: MetadataManager = Depends(get_metadata_manager),
) -> FileMetadataResponse:
    """Get metadata for a specific file."""
    metadata = await manager.get_metadata(path)

    if metadata is None:
        raise HTTPException(status_code=404, detail=f"Metadata not found: {path}")

    return FileMetadataResponse(
        path=metadata.path,
        content_hash=metadata.content_hash,
        file_size=metadata.file_size,
        mod_time=metadata.mod_time,
        index_status=IndexStatus(metadata.index_status.value),
        indexed_at=metadata.indexed_at,
        error_message=metadata.error_message,
    )


@router.post("/scan", response_model=ScanResponse)
async def scan_files(
    manager: MetadataManager = Depends(get_metadata_manager),
) -> ScanResponse:
    """Manually trigger directory scan."""
    changes = await manager.scan_and_update()

    return ScanResponse(
        changes=[
            FileChangeResponse(
                path=c.path,
                change_type=c.change_type,
                old_hash=c.old_hash,
                new_hash=c.new_hash,
                timestamp=c.timestamp,
            )
            for c in changes
        ],
        total_changes=len(changes),
    )