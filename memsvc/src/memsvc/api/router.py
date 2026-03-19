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
    ReEmbedRequest,
    ReEmbedResponse,
    SearchMemoryRequest,
    SearchMemoryResponse,
    MemoryResult,
    TriggerMemoryRequest,
    TriggerMemoryResponse,
    LoadMemoryRequest,
    LoadMemoryResponse,
    MemoryContent,
)
from memsvc.config import settings
from memsvc.core.metadata import MetadataManager
from memsvc.core.watcher import FileWatcher
from memsvc.core.indexer import MemoryIndexer

router = APIRouter()

# Global instances (set by app lifespan or tests)
_metadata_manager: MetadataManager | None = None
_file_watcher: FileWatcher | None = None
_memory_indexer: MemoryIndexer | None = None


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


@router.post("/re_embed", response_model=ReEmbedResponse)
async def re_embed(
    request: ReEmbedRequest = ReEmbedRequest(),
    manager: MetadataManager = Depends(get_metadata_manager),
    indexer: MemoryIndexer = Depends(get_memory_indexer),
) -> ReEmbedResponse:
    """Re-index pending memory files.

    Indexes files with PENDING status into ChromaDB vector store.
    Only processes new or modified files (based on content hash).
    """
    # Get pending files
    pending_files = await manager.list_pending()

    # Filter by types if specified
    if request.types:
        pending_files = [
            f for f in pending_files
            if f.path.split("/")[0] in request.types
        ]

    if not pending_files:
        return ReEmbedResponse(
            updated_files=[],
            indexed_count=0,
            skipped_count=0,
        )

    # Index files
    paths = [f.path for f in pending_files]
    results = await indexer.index_files(paths)

    # Process results
    updated_files = []
    errors = []
    indexed_count = 0

    for result in results:
        if result.success:
            updated_files.append(result.path)
            indexed_count += result.chunks_indexed
        else:
            errors.append(f"{result.path}: {result.error}")

    return ReEmbedResponse(
        updated_files=updated_files,
        indexed_count=len(updated_files),
        skipped_count=len(paths) - len(updated_files),
        errors=errors if errors else None,
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