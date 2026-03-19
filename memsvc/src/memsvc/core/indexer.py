"""Memory indexer orchestrating embedding and vector storage."""

import logging
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Any

from memsvc.config import settings
from memsvc.core.chunker import chunk_text
from memsvc.core.embedding import EmbeddingProvider, get_embedding_provider
from memsvc.core.metadata import MetadataManager
from memsvc.core.parser import parse_memory_file
from memsvc.core.vectorstore import VectorStore
from memsvc.models.file_meta import IndexStatus

logger = logging.getLogger(__name__)


@dataclass
class IndexResult:
    """Result of indexing a file."""

    path: str
    success: bool
    error: str | None = None
    chunks_indexed: int = 0


class MemoryIndexer:
    """Orchestrates indexing of memory files.

    Handles:
    - Reading and parsing memory files
    - Chunking full content
    - Generating embeddings
    - Storing in dual ChromaDB collections
    """

    def __init__(
        self,
        memory_dir: Path | None = None,
        vector_store: VectorStore | None = None,
        metadata_manager: MetadataManager | None = None,
        embedding_provider: EmbeddingProvider | None = None,
    ):
        """Initialize memory indexer.

        Args:
            memory_dir: Memory directory path. Defaults to settings.memory_dir.
            vector_store: Vector store instance. Created if not provided.
            metadata_manager: Metadata manager instance.
            embedding_provider: Embedding provider. Created if not provided.
        """
        self.memory_dir = memory_dir or settings.memory_dir
        self.vector_store = vector_store
        self.metadata_manager = metadata_manager
        self.embedding_provider = embedding_provider or get_embedding_provider()

    def initialize(self) -> None:
        """Initialize the indexer and its dependencies."""
        if not self.vector_store:
            self.vector_store = VectorStore(embedding_provider=self.embedding_provider)
            self.vector_store.initialize()

    async def index_file(self, relative_path: str) -> IndexResult:
        """Index a single memory file.

        Args:
            relative_path: Relative path from memory_dir.

        Returns:
            IndexResult with success status.
        """
        file_path = self.memory_dir / relative_path

        try:
            # Read file content
            content = file_path.read_text(encoding="utf-8")

            # Parse frontmatter
            parsed = parse_memory_file(content)

            # Prepare description for trigger collection
            description = parsed.description or f"Memory file: {relative_path}"

            # Add to trigger collection (single doc per file)
            trigger_metadata = {
                "source_path": relative_path,
                "file_type": relative_path.split("/")[0] if "/" in relative_path else "unknown",
                "indexed_at": datetime.now().isoformat(),
            }

            await self.vector_store.add_to_trigger(
                ids=[relative_path],
                texts=[description],
                metadatas=[trigger_metadata],
            )

            # Chunk and add to search collection
            full_text = f"{description}\n\n{parsed.body}" if description and parsed.body else description or parsed.body
            chunks = chunk_text(full_text)

            chunk_ids = []
            chunk_texts = []
            chunk_metadatas = []

            for i, chunk in enumerate(chunks):
                chunk_id = f"{relative_path}#chunk_{i}"
                chunk_ids.append(chunk_id)
                chunk_texts.append(chunk)
                chunk_metadatas.append({
                    "source_path": relative_path,
                    "chunk_index": i,
                    "description": description,
                    "indexed_at": datetime.now().isoformat(),
                })

            if chunk_ids:
                await self.vector_store.add_to_search(
                    ids=chunk_ids,
                    texts=chunk_texts,
                    metadatas=chunk_metadatas,
                )

            # Update metadata status
            if self.metadata_manager:
                await self.metadata_manager.mark_indexed(relative_path)

            logger.info(f"Indexed {relative_path}: {len(chunks)} chunks")

            return IndexResult(
                path=relative_path,
                success=True,
                chunks_indexed=len(chunks),
            )

        except Exception as e:
            error_msg = str(e)
            logger.error(f"Failed to index {relative_path}: {error_msg}")

            # Update metadata status
            if self.metadata_manager:
                await self.metadata_manager.mark_failed(relative_path, error_msg)

            return IndexResult(
                path=relative_path,
                success=False,
                error=error_msg,
            )

    async def index_files(self, paths: list[str]) -> list[IndexResult]:
        """Index multiple files.

        Args:
            paths: List of relative paths to index.

        Returns:
            List of IndexResult for each file.
        """
        results = []
        for path in paths:
            result = await self.index_file(path)
            results.append(result)
        return results

    async def remove_from_index(self, relative_path: str) -> None:
        """Remove a file from both collections.

        Args:
            relative_path: Relative path of the file to remove.
        """
        if self.vector_store:
            self.vector_store.delete(relative_path)
            logger.info(f"Removed {relative_path} from index")

    async def reindex_all(self) -> list[IndexResult]:
        """Reindex all pending files.

        Returns:
            List of IndexResult for each file.
        """
        if not self.metadata_manager:
            raise RuntimeError("Metadata manager not set")

        pending = await self.metadata_manager.list_pending()
        paths = [f.path for f in pending]

        logger.info(f"Reindexing {len(paths)} pending files")
        return await self.index_files(paths)

    async def load_memory(self, ids: list[str]) -> list[dict[str, Any]]:
        """Load full memory content by IDs (file paths).

        Args:
            ids: List of file paths to load.

        Returns:
            List of memory content dictionaries.
        """
        memories = []

        for file_id in ids:
            # Extract the source path (remove #chunk_X suffix if present)
            source_path = file_id.split("#chunk_")[0] if "#chunk_" in file_id else file_id
            file_path = self.memory_dir / source_path

            try:
                if file_path.exists():
                    content = file_path.read_text(encoding="utf-8")
                    parsed = parse_memory_file(content)

                    memories.append({
                        "id": source_path,
                        "content": content,
                        "source": source_path,
                        "description": parsed.description,
                    })
            except Exception as e:
                logger.warning(f"Failed to load {source_path}: {e}")

        return memories