"""Embedding providers for memory indexing."""

import hashlib
import logging
import math
from abc import ABC, abstractmethod
from http import HTTPStatus

import dashscope
from dashscope import TextEmbedding

from memsvc.config import settings

logger = logging.getLogger(__name__)


class EmbeddingProvider(ABC):
    """Abstract base class for embedding providers."""

    @abstractmethod
    async def embed(self, texts: list[str]) -> list[list[float]]:
        """Generate embeddings for texts.

        Args:
            texts: List of text strings to embed.

        Returns:
            List of embedding vectors.
        """
        pass

    @property
    @abstractmethod
    def dimension(self) -> int:
        """Return embedding dimension."""
        pass


class QwenEmbedding(EmbeddingProvider):
    """Qwen embedding via DashScope SDK."""

    def __init__(
        self,
        api_key: str | None = None,
        model: str | None = None,
    ):
        """Initialize Qwen embedding provider.

        Args:
            api_key: DashScope API key. Defaults to settings.embedding_api_key.
            model: Model name. Defaults to settings.embedding_model.
        """
        self.api_key = api_key or settings.embedding_api_key
        self.model = model or settings.embedding_model
        self._dimension = settings.embedding_dimension

        if not self.api_key:
            raise ValueError("Qwen embedding requires an API key. Set MEMSVC_EMBEDDING_API_KEY.")

        # Configure dashscope with API key
        dashscope.api_key = self.api_key

    @property
    def dimension(self) -> int:
        return self._dimension

    async def embed(self, texts: list[str]) -> list[list[float]]:
        """Generate embeddings using DashScope SDK.

        Args:
            texts: List of text strings to embed.

        Returns:
            List of embedding vectors.
        """
        # Use sync call in async context - dashscope doesn't have native async
        # For production, consider using asyncio.to_thread for better concurrency
        import asyncio

        def _call_embedding():
            response = TextEmbedding.call(
                model=self.model,
                input=texts,
            )
            return response

        response = await asyncio.to_thread(_call_embedding)

        if response.status_code != HTTPStatus.OK:
            raise RuntimeError(f"DashScope API error: {response.code} - {response.message}")

        # Extract embeddings from response
        embeddings = []
        for item in response.output["embeddings"]:
            embeddings.append(item["embedding"])

        return embeddings


class MockEmbedding(EmbeddingProvider):
    """Mock embedding provider for testing without external dependencies.

    Generates deterministic embeddings based on text hash.
    Not suitable for production use.
    """

    def __init__(self, dimension: int = 1024):
        """Initialize mock embedding provider.

        Args:
            dimension: Embedding dimension. Defaults to 1024.
        """
        self._dimension = dimension

    @property
    def dimension(self) -> int:
        return self._dimension

    async def embed(self, texts: list[str]) -> list[list[float]]:
        """Generate deterministic mock embeddings.

        Uses SHA256 hash of text to generate pseudo-random but deterministic
        embeddings. Useful for testing without external API calls.

        Args:
            texts: List of text strings to embed.

        Returns:
            List of mock embedding vectors.
        """
        embeddings = []
        for text in texts:
            # Use hash to generate deterministic values
            hash_bytes = hashlib.sha256(text.encode()).digest()
            # Expand to needed bytes (dimension * 4 bytes for float32)
            expanded = hash_bytes * (self._dimension * 4 // len(hash_bytes) + 1)

            # Convert to floats and normalize
            vector = []
            for i in range(self._dimension):
                # Get 4 bytes for each float
                int_val = int.from_bytes(expanded[i * 4 : (i + 1) * 4], "big")
                # Normalize to [-1, 1] range
                vector.append((int_val / (2**32 - 1)) * 2 - 1)

            # Normalize vector to unit length
            norm = math.sqrt(sum(x * x for x in vector))
            if norm > 0:
                vector = [x / norm for x in vector]

            embeddings.append(vector)

        return embeddings


def get_embedding_provider(provider: str | None = None) -> EmbeddingProvider:
    """Factory function to create embedding provider.

    Args:
        provider: Provider name ("qwen" or "mock"). Defaults to settings.embedding_provider.

    Returns:
        EmbeddingProvider instance.

    Raises:
        ValueError: If provider name is unknown.
    """
    provider = provider or settings.embedding_provider

    if provider == "qwen":
        return QwenEmbedding()
    elif provider == "mock":
        return MockEmbedding()
    else:
        raise ValueError(f"Unknown embedding provider: {provider}")