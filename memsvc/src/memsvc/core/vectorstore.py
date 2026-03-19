"""Vector store using ChromaDB with dual collections."""

import logging
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import chromadb
from chromadb.config import Settings as ChromaSettings

from memsvc.config import settings
from memsvc.core.embedding import EmbeddingProvider, get_embedding_provider

logger = logging.getLogger(__name__)

COLLECTION_TRIGGER = "memory_trigger"
COLLECTION_SEARCH = "memory_search"


@dataclass
class SearchResult:
    """Search result from vector store."""

    id: str
    score: float
    text: str
    metadata: dict[str, Any]


@dataclass
class TriggerResult:
    """Result from trigger query."""

    has_relevant: bool
    hint: str | None = None
    score: float = 0.0


class VectorStore:
    """Vector store using ChromaDB with dual collections.

    Collections:
    - memory_trigger: Lightweight descriptions for fast relevance checking
    - memory_search: Chunked full content for detailed semantic search
    """

    def __init__(
        self,
        persist_dir: Path | None = None,
        embedding_provider: EmbeddingProvider | None = None,
    ):
        """Initialize vector store.

        Args:
            persist_dir: Directory for ChromaDB persistence. Defaults to settings.chroma_persist_dir.
            embedding_provider: Embedding provider. Defaults to factory-created provider.
        """
        self.persist_dir = persist_dir or settings.chroma_persist_dir
        self.embedding_provider = embedding_provider or get_embedding_provider()
        self._client: chromadb.Client | None = None
        self._trigger_collection: chromadb.Collection | None = None
        self._search_collection: chromadb.Collection | None = None

    def initialize(self) -> None:
        """Initialize ChromaDB client and collections."""
        # Ensure persistence directory exists
        self.persist_dir.mkdir(parents=True, exist_ok=True)

        # Create persistent client
        self._client = chromadb.PersistentClient(
            path=str(self.persist_dir),
            settings=ChromaSettings(
                anonymized_telemetry=False,
            ),
        )

        # Get or create collections
        self._trigger_collection = self._client.get_or_create_collection(
            name=COLLECTION_TRIGGER,
            metadata={"description": "Lightweight descriptions for trigger queries"},
        )

        self._search_collection = self._client.get_or_create_collection(
            name=COLLECTION_SEARCH,
            metadata={"description": "Chunked content for semantic search"},
        )

        logger.info(
            f"Vector store initialized with {self._trigger_collection.count()} trigger docs, "
            f"{self._search_collection.count()} search docs"
        )

    def close(self) -> None:
        """Close the vector store."""
        self._client = None
        self._trigger_collection = None
        self._search_collection = None

    async def add_to_trigger(
        self,
        ids: list[str],
        texts: list[str],
        metadatas: list[dict[str, Any]] | None = None,
    ) -> None:
        """Add documents to trigger collection.

        Args:
            ids: Document IDs (file paths).
            texts: Text to embed (descriptions).
            metadatas: Optional metadata for each document.
        """
        if not self._trigger_collection:
            raise RuntimeError("Vector store not initialized")

        # Generate embeddings
        embeddings = await self.embedding_provider.embed(texts)

        self._trigger_collection.add(
            ids=ids,
            embeddings=embeddings,
            documents=texts,
            metadatas=metadatas,
        )

        logger.debug(f"Added {len(ids)} documents to trigger collection")

    async def add_to_search(
        self,
        ids: list[str],
        texts: list[str],
        metadatas: list[dict[str, Any]] | None = None,
    ) -> None:
        """Add documents to search collection.

        Args:
            ids: Document IDs (chunk IDs: {file_path}#chunk_{index}).
            texts: Text to embed (chunks).
            metadatas: Optional metadata for each document.
        """
        if not self._search_collection:
            raise RuntimeError("Vector store not initialized")

        # Generate embeddings
        embeddings = await self.embedding_provider.embed(texts)

        self._search_collection.add(
            ids=ids,
            embeddings=embeddings,
            documents=texts,
            metadatas=metadatas,
        )

        logger.debug(f"Added {len(ids)} documents to search collection")

    def delete(self, file_id: str) -> None:
        """Delete a file from both collections.

        Args:
            file_id: File path to delete.
        """
        if not self._client:
            raise RuntimeError("Vector store not initialized")

        # Delete from trigger collection
        try:
            self._trigger_collection.delete(ids=[file_id])
            logger.debug(f"Deleted {file_id} from trigger collection")
        except Exception as e:
            logger.warning(f"Error deleting from trigger collection: {e}")

        # Delete all chunks from search collection
        # Query for all chunks with matching source_path
        try:
            # Get all chunk IDs for this file
            existing = self._search_collection.get(
                where={"source_path": file_id},
            )
            if existing["ids"]:
                self._search_collection.delete(ids=existing["ids"])
                logger.debug(f"Deleted {len(existing['ids'])} chunks from search collection")
        except Exception as e:
            logger.warning(f"Error deleting chunks from search collection: {e}")

    async def query_trigger(
        self,
        query: str,
        n_results: int = 5,
        threshold: float = 0.7,
    ) -> TriggerResult:
        """Query trigger collection for relevance check.

        Args:
            query: Query text.
            n_results: Number of results to retrieve.
            threshold: Similarity threshold for relevance.

        Returns:
            TriggerResult with has_relevant boolean and hint.
        """
        if not self._trigger_collection:
            raise RuntimeError("Vector store not initialized")

        # Generate query embedding
        embeddings = await self.embedding_provider.embed([query])

        # Query collection
        results = self._trigger_collection.query(
            query_embeddings=embeddings,
            n_results=n_results,
            include=["documents", "distances", "metadatas"],
        )

        if not results["ids"][0]:
            return TriggerResult(has_relevant=False)

        # Convert distance to similarity score (ChromaDB returns L2 distance)
        # For normalized vectors, distance = 2 * (1 - similarity)
        best_distance = results["distances"][0][0]
        best_score = 1.0 - (best_distance / 2.0)

        if best_score >= threshold:
            hint = results["documents"][0][0] if results["documents"][0] else None
            return TriggerResult(
                has_relevant=True,
                hint=hint,
                score=best_score,
            )

        return TriggerResult(has_relevant=False, score=best_score)

    async def query_search(
        self,
        queries: list[str],
        n_results: int = 5,
    ) -> list[list[SearchResult]]:
        """Query search collection for semantic search.

        Args:
            queries: List of query texts.
            n_results: Number of results per query.

        Returns:
            List of search results for each query.
        """
        if not self._search_collection:
            raise RuntimeError("Vector store not initialized")

        # Generate query embeddings
        embeddings = await self.embedding_provider.embed(queries)

        # Query collection
        results = self._search_collection.query(
            query_embeddings=embeddings,
            n_results=n_results,
            include=["documents", "distances", "metadatas"],
        )

        # Process results
        all_results = []
        for i in range(len(queries)):
            query_results = []
            if results["ids"][i]:
                for j, doc_id in enumerate(results["ids"][i]):
                    distance = results["distances"][i][j]
                    score = 1.0 - (distance / 2.0)  # Convert distance to similarity

                    query_results.append(
                        SearchResult(
                            id=doc_id,
                            score=score,
                            text=results["documents"][i][j],
                            metadata=results["metadatas"][i][j] if results["metadatas"][i] else {},
                        )
                    )
            all_results.append(query_results)

        return all_results

    def count(self) -> dict[str, int]:
        """Get document counts for both collections.

        Returns:
            Dictionary with trigger_count and search_count.
        """
        return {
            "trigger_count": self._trigger_collection.count() if self._trigger_collection else 0,
            "search_count": self._search_collection.count() if self._search_collection else 0,
        }