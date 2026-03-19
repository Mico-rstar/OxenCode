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
    SearchMemoryRequest,
    SearchMemoryResponse,
    MemoryResult,
    TriggerMemoryRequest,
    TriggerMemoryResponse,
    LoadMemoryRequest,
    LoadMemoryResponse,
    MemoryContent,
    # Session API schemas
    CommitSessionRequest,
    CommitSessionResponse,
    TaskStatusResponse,
    RetrySessionRequest,
)
from memsvc.config import settings
from memsvc.core.metadata import MetadataManager
from memsvc.core.watcher import FileWatcher
from memsvc.core.indexer import MemoryIndexer
from memsvc.core.task_manager import TaskManager
from memsvc.core.compressor import SessionCompressor

router = APIRouter()

# Global instances (set by app lifespan or tests)
_metadata_manager: MetadataManager | None = None
_file_watcher: FileWatcher | None = None
_memory_indexer: MemoryIndexer | None = None
_task_manager: TaskManager | None = None
_session_compressor: SessionCompressor | None = None


def set_metadata_manager(manager: MetadataManager | None) -> None:
    """Set metadata manager instance (used by app lifespan or tests)."""
    global _metadata_manager
    _metadata_manager = manager


def set_file_watcher(watcher: FileWatcher | None) -> None:
    """Set file watcher instance (used by app lifespan or tests)."""
    global _file_watcher
    _file_watcher = watcher


def set_memory_indexer(indexer: MemoryIndexer | None) -> None:
    """Set memory indexer instance (used by app lifespan or tests)."""
    global _memory_indexer
    _memory_indexer = indexer


def set_task_manager(manager: TaskManager | None) -> None:
    """Set task manager instance (used by app lifespan or tests)."""
    global _task_manager
    _task_manager = manager


def set_session_compressor(compressor: SessionCompressor | None) -> None:
    """Set session compressor instance (used by app lifespan or tests)."""
    global _session_compressor
    _session_compressor = compressor


def get_metadata_manager() -> MetadataManager:
    """Dependency to get metadata manager instance."""
    if _metadata_manager is None:
        raise HTTPException(status_code=503, detail="Service not initialized")
    return _metadata_manager


def get_file_watcher() -> FileWatcher | None:
    """Dependency to get file watcher instance."""
    return _file_watcher


def get_memory_indexer() -> MemoryIndexer:
    """Dependency to get memory indexer instance."""
    if _memory_indexer is None:
        raise HTTPException(status_code=503, detail="Memory indexer not initialized")
    return _memory_indexer


def get_task_manager() -> TaskManager:
    """Dependency to get task manager instance."""
    if _task_manager is None:
        raise HTTPException(status_code=503, detail="Task manager not initialized")
    return _task_manager


def get_session_compressor() -> SessionCompressor:
    """Dependency to get session compressor instance."""
    if _session_compressor is None:
        raise HTTPException(status_code=503, detail="Session compressor not initialized")
    return _session_compressor


@router.get("/health", response_model=HealthResponse)
async def health_check() -> HealthResponse:
    """Health check endpoint."""
    return HealthResponse()


@router.get("/status", response_model=ServiceStatusResponse)
async def get_status(
    manager: MetadataManager = Depends(get_metadata_manager),
    watcher: FileWatcher | None = Depends(get_file_watcher),
    indexer: MemoryIndexer | None = Depends(lambda: _memory_indexer),
) -> ServiceStatusResponse:
    """Get service status."""
    counts = await manager.count_by_status()
    total = sum(counts.values())

    # Get vector store counts
    trigger_docs = 0
    search_docs = 0
    if indexer and indexer.vector_store:
        vs_counts = indexer.vector_store.count()
        trigger_docs = vs_counts.get("trigger_count", 0)
        search_docs = vs_counts.get("search_count", 0)

    return ServiceStatusResponse(
        watch_enabled=settings.watch_enabled,
        watch_active=watcher.is_active if watcher else False,
        total_files=total,
        pending_count=counts.get(IndexStatus.PENDING.value, 0),
        indexed_count=counts.get(IndexStatus.INDEXED.value, 0),
        failed_count=counts.get(IndexStatus.FAILED.value, 0),
        memory_dir=str(settings.memory_dir),
        uptime_seconds=manager.get_uptime(),
        trigger_docs=trigger_docs,
        search_docs=search_docs,
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


@router.post("/search_memory", response_model=SearchMemoryResponse)
async def search_memory(
    request: SearchMemoryRequest,
    indexer: MemoryIndexer = Depends(get_memory_indexer),
) -> SearchMemoryResponse:
    """Search memory using semantic similarity.

    Searches the memory_search collection for relevant content.
    Returns matching chunks with description and excerpt.
    """
    if not indexer.vector_store:
        raise HTTPException(status_code=503, detail="Vector store not initialized")

    # Query vector store
    results = await indexer.vector_store.query_search(
        queries=request.queries,
        n_results=request.top_k,
    )

    # Flatten and deduplicate results
    seen_ids = set()
    memory_results = []

    for query_results in results:
        for result in query_results:
            if result.id not in seen_ids:
                seen_ids.add(result.id)
                memory_results.append(
                    MemoryResult(
                        id=result.metadata.get("source_path", result.id),
                        description=result.metadata.get("description", ""),
                        score=result.score,
                        excerpt=result.text[:500] if len(result.text) > 500 else result.text,
                    )
                )

    return SearchMemoryResponse(results=memory_results[: request.top_k * len(request.queries)])


@router.post("/trigger_memory", response_model=TriggerMemoryResponse)
async def trigger_memory(
    request: TriggerMemoryRequest,
    indexer: MemoryIndexer = Depends(get_memory_indexer),
) -> TriggerMemoryResponse:
    """Quick check if relevant memory exists.

    Queries the memory_trigger collection for fast relevance checking.
    Returns boolean and hint if relevant memory found.
    """
    if not indexer.vector_store:
        raise HTTPException(status_code=503, detail="Vector store not initialized")

    result = await indexer.vector_store.query_trigger(
        query=request.query,
        n_results=1,
        threshold=request.threshold,
    )

    return TriggerMemoryResponse(
        has_relevant=result.has_relevant,
        hint=result.hint,
        score=result.score,
    )


@router.post("/load_memory", response_model=LoadMemoryResponse)
async def load_memory(
    request: LoadMemoryRequest,
    indexer: MemoryIndexer = Depends(get_memory_indexer),
) -> LoadMemoryResponse:
    """Load full memory content by IDs.

    Loads the complete content of memory files specified by their IDs (paths).
    """
    memories = await indexer.load_memory(request.ids)

    return LoadMemoryResponse(
        memories=[
            MemoryContent(
                id=m["id"],
                content=m["content"],
                source=m["source"],
                description=m.get("description"),
            )
            for m in memories
        ]
    )


# === Session API Endpoints ===


@router.post("/commit_session", response_model=CommitSessionResponse)
async def commit_session(
    request: CommitSessionRequest,
    task_manager: TaskManager = Depends(get_task_manager),
) -> CommitSessionResponse:
    """Commit a session for async processing.

    Writes messages to histories and compresses to notes asynchronously.
    Returns immediately with a task_id for tracking.

    Args:
        request: Session ID and messages to process.

    Returns:
        task_id: Unique identifier for tracking the async task.
    """
    # Convert messages to dict format
    messages = [
        {
            "role": msg.role,
            "content": msg.content,
            "timestamp": msg.timestamp.isoformat() if msg.timestamp else None,
        }
        for msg in request.messages
    ]

    try:
        task_id = await task_manager.create_task(request.session_id, messages)
        return CommitSessionResponse(task_id=task_id)
    except ValueError as e:
        raise HTTPException(status_code=409, detail=str(e))


@router.get(
    "/task/{task_id}/status",
    response_model=TaskStatusResponse,
    responses={404: {"model": ErrorResponse}},
)
async def get_task_status(
    task_id: str,
    task_manager: TaskManager = Depends(get_task_manager),
) -> TaskStatusResponse:
    """Get the status of an async task.

    Args:
        task_id: Task identifier returned by commit_session.

    Returns:
        Current task status and metadata.
    """
    task = await task_manager.get_task(task_id)
    if task is None:
        raise HTTPException(status_code=404, detail=f"Task not found: {task_id}")

    return TaskStatusResponse(
        task_id=task.task_id,
        session_id=task.session_id,
        status=task.status.value,
        created_at=task.created_at,
        updated_at=task.updated_at,
        error_message=task.error_message,
        histories_written=task.histories_written,
    )


@router.post("/retry_session", response_model=CommitSessionResponse)
async def retry_session(
    request: RetrySessionRequest,
    task_manager: TaskManager = Depends(get_task_manager),
    compressor: SessionCompressor = Depends(get_session_compressor),
) -> CommitSessionResponse:
    """Retry a failed session processing.

    Re-processes a session that has histories but failed to compress.
    Does not re-write histories if they already exist.

    Args:
        request: Session ID to retry.

    Returns:
        task_id: New task identifier for tracking.

    Raises:
        404: If session has no histories to process.
        409: If session already has a pending task.
    """
    session_id = request.session_id

    # Check if histories exist
    if not compressor.histories_exists(session_id):
        raise HTTPException(
            status_code=404,
            detail=f"No histories found for session: {session_id}"
        )

    # Load messages from histories
    session_data = compressor.load_histories(session_id)
    if not session_data:
        raise HTTPException(
            status_code=404,
            detail=f"Failed to load histories for session: {session_id}"
        )

    messages = session_data.get("messages", [])

    # Check for existing task
    existing = await task_manager.get_task_by_session(session_id)
    if existing and existing.status in ("pending", "running"):
        raise HTTPException(
            status_code=409,
            detail=f"Session {session_id} already has a {existing.status} task"
        )

    # Create new task - the processor will skip histories since they exist
    task_id = await task_manager.create_task(session_id, messages)

    # Mark that histories are already written (for the retry case)
    task = await task_manager.get_task(task_id)
    if task:
        task.histories_written = True

    return CommitSessionResponse(task_id=task_id)