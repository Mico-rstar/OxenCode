"""File watcher using watchdog."""

import asyncio
import logging
from datetime import datetime
from pathlib import Path
from typing import TYPE_CHECKING

from watchdog.events import FileSystemEvent, FileSystemEventHandler
from watchdog.observers import Observer

from memsvc.config import settings
from memsvc.models.file_meta import FileMetadata, IndexStatus

if TYPE_CHECKING:
    from memsvc.core.metadata import MetadataManager

logger = logging.getLogger(__name__)


class MemoryFileHandler(FileSystemEventHandler):
    """Handler for memory file system events."""

    def __init__(
        self,
        metadata_manager: "MetadataManager",
        memory_dir: Path,
        loop: asyncio.AbstractEventLoop | None = None,
    ):
        super().__init__()
        self.metadata_manager = metadata_manager
        self.memory_dir = memory_dir
        self.loop = loop or asyncio.get_event_loop()
        self._change_queue: asyncio.Queue[dict] = asyncio.Queue()

    def on_created(self, event: FileSystemEvent) -> None:
        """Handle file creation event."""
        if event.is_directory:
            return
        self._handle_file_change(event.src_path, "created")

    def on_modified(self, event: FileSystemEvent) -> None:
        """Handle file modification event."""
        if event.is_directory:
            return
        self._handle_file_change(event.src_path, "modified")

    def on_deleted(self, event: FileSystemEvent) -> None:
        """Handle file deletion event."""
        if event.is_directory:
            return
        path = Path(event.src_path)

        # Check if in watched directories
        try:
            rel_path = path.relative_to(self.memory_dir)
            parts = rel_path.parts
            if len(parts) > 0 and parts[0] not in settings.watch_dirs:
                return
        except ValueError:
            return

        # Check extension
        if path.suffix not in settings.supported_extensions:
            return

        rel_path_str = str(rel_path)
        logger.info(f"File deleted: {rel_path_str}")

        # Schedule async task to mark deleted
        asyncio.run_coroutine_threadsafe(
            self.metadata_manager.mark_deleted(rel_path_str), self.loop
        )

    def on_moved(self, event: FileSystemEvent) -> None:
        """Handle file move event."""
        if event.is_directory:
            return
        # Treat as delete from old location and create at new location
        self.on_deleted(type("Event", (), {"is_directory": False, "src_path": event.src_path})())
        self.on_created(type("Event", (), {"is_directory": False, "src_path": event.dest_path})())

    def _handle_file_change(self, src_path: str, change_type: str) -> None:
        """Process a file change event."""
        path = Path(src_path)

        # Check if in watched directories
        try:
            rel_path = path.relative_to(self.memory_dir)
            parts = rel_path.parts
            if len(parts) > 0 and parts[0] not in settings.watch_dirs:
                return
        except ValueError:
            return

        # Check extension
        if path.suffix not in settings.supported_extensions:
            return

        # Check file size
        try:
            if path.stat().st_size > settings.max_file_size:
                logger.warning(f"File too large, skipping: {rel_path}")
                return
        except FileNotFoundError:
            return

        rel_path_str = str(rel_path)
        logger.info(f"File {change_type}: {rel_path_str}")

        # Schedule async task to update metadata
        asyncio.run_coroutine_threadsafe(
            self._update_file_metadata(path, rel_path_str), self.loop
        )

    async def _update_file_metadata(self, path: Path, rel_path: str) -> None:
        """Update metadata for a file."""
        from memsvc.core.metadata import compute_hash

        try:
            content_hash = compute_hash(path)
            file_stat = path.stat()

            await self.metadata_manager.upsert_metadata(
                FileMetadata(
                    path=rel_path,
                    content_hash=content_hash,
                    file_size=file_stat.st_size,
                    mod_time=datetime.fromtimestamp(file_stat.st_mtime),
                    index_status=IndexStatus.PENDING,
                )
            )
        except Exception as e:
            logger.error(f"Error updating metadata for {rel_path}: {e}")


class FileWatcher:
    """File watcher for memory directory."""

    def __init__(self, metadata_manager: "MetadataManager"):
        self.metadata_manager = metadata_manager
        self.memory_dir = settings.memory_dir
        self.watch_dirs = settings.watch_dirs
        self._observer: Observer | None = None
        self._handler: MemoryFileHandler | None = None
        self._loop: asyncio.AbstractEventLoop | None = None
        self._active = False

    @property
    def is_active(self) -> bool:
        """Check if watcher is active."""
        return self._active

    def start(self, loop: asyncio.AbstractEventLoop | None = None) -> None:
        """Start watching memory directories."""
        if self._observer is not None:
            logger.warning("Watcher already running")
            return

        self._loop = loop or asyncio.get_event_loop()
        self._handler = MemoryFileHandler(
            self.metadata_manager, self.memory_dir, self._loop
        )
        self._observer = Observer()

        # Watch each configured directory
        for watch_dir in self.watch_dirs:
            watch_path = self.memory_dir / watch_dir
            watch_path.mkdir(parents=True, exist_ok=True)
            self._observer.schedule(self._handler, str(watch_path), recursive=True)
            logger.info(f"Watching directory: {watch_path}")

        self._observer.start()
        self._active = True
        logger.info("File watcher started")

    def stop(self) -> None:
        """Stop watching."""
        if self._observer:
            self._observer.stop()
            self._observer.join(timeout=5.0)
            self._observer = None
            self._handler = None
            self._active = False
            logger.info("File watcher stopped")