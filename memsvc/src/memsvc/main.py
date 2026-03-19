"""FastAPI application entry point."""

import asyncio
import logging

import uvicorn
from fastapi import FastAPI
from contextlib import asynccontextmanager

from memsvc import __version__
from memsvc.api import router
from memsvc.api.router import (
    set_metadata_manager,
    set_file_watcher,
    set_memory_indexer,
    set_task_manager,
    set_session_compressor,
)
from memsvc.config import settings
from memsvc.core.metadata import MetadataManager
from memsvc.core.watcher import FileWatcher
from memsvc.core.indexer import MemoryIndexer
from memsvc.core.llm import get_llm_provider, LLMProvider
from memsvc.core.task_manager import TaskManager
from memsvc.core.compressor import SessionCompressor

# Configure logging
logging.basicConfig(
    level=getattr(logging, settings.log_level.upper()),
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)

# Global instances
metadata_manager: MetadataManager | None = None
file_watcher: FileWatcher | None = None
memory_indexer: MemoryIndexer | None = None
llm_provider: LLMProvider | None = None
task_manager: TaskManager | None = None
session_compressor: SessionCompressor | None = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan manager."""
    global metadata_manager, file_watcher, memory_indexer, llm_provider, task_manager, session_compressor

    # Startup
    logger.info("Starting memory service...")
    settings.ensure_directories()

    # Initialize metadata manager
    metadata_manager = MetadataManager()
    await metadata_manager.initialize()
    logger.info("Metadata manager initialized")

    # Inject into router
    set_metadata_manager(metadata_manager)

    # Initial scan
    changes = await metadata_manager.scan_and_update()
    logger.info(f"Initial scan complete: {len(changes)} changes detected")

    # Initialize memory indexer
    memory_indexer = MemoryIndexer(
        metadata_manager=metadata_manager,
    )
    memory_indexer.initialize()
    set_memory_indexer(memory_indexer)
    logger.info("Memory indexer initialized")

    # Initialize LLM provider
    llm_provider = get_llm_provider()
    logger.info(f"LLM provider initialized: {llm_provider.model}")

    # Initialize session compressor
    session_compressor = SessionCompressor(llm=llm_provider)
    set_session_compressor(session_compressor)
    logger.info("Session compressor initialized")

    # Initialize task manager
    task_manager = TaskManager(metadata_manager=metadata_manager)
    await task_manager.initialize()

    # Set the processor for task manager
    async def process_session_task(task, messages):
        await session_compressor.process_session(
            task,
            messages,
            skip_histories=task.histories_written,
        )

    task_manager.set_processor(process_session_task)
    set_task_manager(task_manager)
    logger.info("Task manager initialized")

    # Recover pending tasks from previous run
    recovered = await task_manager.recover_tasks()
    if recovered:
        logger.info(f"Recovered {len(recovered)} pending tasks")

    # Start file watcher if enabled
    if settings.watch_enabled:
        file_watcher = FileWatcher(metadata_manager)
        file_watcher.start()
        set_file_watcher(file_watcher)
        logger.info("File watcher started")

    yield

    # Shutdown
    logger.info("Shutting down memory service...")

    if task_manager:
        await task_manager.close()
        logger.info("Task manager closed")

    if file_watcher:
        file_watcher.stop()
        logger.info("File watcher stopped")

    if memory_indexer:
        if memory_indexer.vector_store:
            memory_indexer.vector_store.close()
        logger.info("Memory indexer closed")

    if metadata_manager:
        await metadata_manager.close()
        logger.info("Metadata manager closed")

    # Clear global references
    set_metadata_manager(None)
    set_file_watcher(None)
    set_memory_indexer(None)
    set_task_manager(None)
    set_session_compressor(None)


app = FastAPI(
    title="OxenCode Memory Service",
    description="File monitoring and metadata management for OxenCode memory system",
    version=__version__,
    lifespan=lifespan,
)

app.include_router(router.router, prefix="/api/v1")


def main():
    """Run the server."""
    uvicorn.run(
        "memsvc.main:app",
        host=settings.host,
        port=settings.port,
        reload=False,
        log_level=settings.log_level,
    )


if __name__ == "__main__":
    main()