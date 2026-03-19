"""Tests for indexing functionality."""

import tempfile
from pathlib import Path

import pytest

from memsvc.core.chunker import chunk_text, get_text_splitter
from memsvc.core.parser import parse_memory_file, extract_description, get_full_text_for_indexing
from memsvc.core.embedding import MockEmbedding, get_embedding_provider
from memsvc.core.vectorstore import VectorStore


class TestChunker:
    """Tests for text chunker."""

    def test_chunk_text_simple(self):
        """Test chunking simple text."""
        text = "Hello world. This is a test. " * 100
        chunks = chunk_text(text, chunk_size=100, chunk_overlap=10)

        assert len(chunks) > 1
        for chunk in chunks:
            assert len(chunk) <= 150  # Allow some flexibility for word boundaries

    def test_chunk_text_chinese(self):
        """Test chunking Chinese text."""
        text = "这是第一句话。这是第二句话。这是第三句话。" * 50
        chunks = chunk_text(text, chunk_size=100, chunk_overlap=10)

        assert len(chunks) > 1

    def test_get_text_splitter_returns_splitter(self):
        """Test getting text splitter."""
        splitter = get_text_splitter(chunk_size=200, chunk_overlap=20)
        assert splitter is not None
        assert splitter._chunk_size == 200


class TestParser:
    """Tests for frontmatter parser."""

    def test_parse_with_frontmatter(self):
        """Test parsing file with frontmatter."""
        content = """---
description: This is a test memory
tags: test, example
---

# Main Content

This is the body of the document.
"""
        parsed = parse_memory_file(content)

        assert parsed.description == "This is a test memory"
        assert parsed.metadata.get("tags") == "test, example"
        assert "# Main Content" in parsed.body

    def test_parse_without_frontmatter(self):
        """Test parsing file without frontmatter."""
        content = """# Simple Document

No frontmatter here.
"""
        parsed = parse_memory_file(content)

        assert parsed.description == ""
        assert "# Simple Document" in parsed.body

    def test_extract_description(self):
        """Test extracting description."""
        content = """---
description: My description
---

Body content.
"""
        desc = extract_description(content)
        assert desc == "My description"

    def test_get_full_text_for_indexing(self):
        """Test getting full text for indexing."""
        content = """---
description: A description
---

Body content here.
"""
        full_text = get_full_text_for_indexing(content)
        assert "A description" in full_text
        assert "Body content here" in full_text


class TestEmbedding:
    """Tests for embedding providers."""

    @pytest.mark.asyncio
    async def test_mock_embedding(self):
        """Test mock embedding provider."""
        provider = MockEmbedding(dimension=128)

        assert provider.dimension == 128

        texts = ["Hello", "World"]
        embeddings = await provider.embed(texts)

        assert len(embeddings) == 2
        assert len(embeddings[0]) == 128
        # Mock embeddings should be normalized
        import math
        norm = math.sqrt(sum(x * x for x in embeddings[0]))
        assert abs(norm - 1.0) < 0.001

    def test_get_embedding_provider_mock(self):
        """Test factory returns mock provider."""
        provider = get_embedding_provider("mock")
        assert isinstance(provider, MockEmbedding)


class TestVectorStore:
    """Tests for vector store."""

    @pytest.fixture
    def temp_vector_store(self, tmp_path: Path):
        """Create a temporary vector store."""
        store = VectorStore(
            persist_dir=tmp_path / "chromadb",
            embedding_provider=MockEmbedding(dimension=128),
        )
        store.initialize()
        yield store
        store.close()

    @pytest.mark.asyncio
    async def test_add_and_query_trigger(self, temp_vector_store: VectorStore):
        """Test adding to and querying trigger collection."""
        await temp_vector_store.add_to_trigger(
            ids=["test1.md"],
            texts=["This is a test description"],
            metadatas=[{"source_path": "test1.md"}],
        )

        result = await temp_vector_store.query_trigger(
            query="test description",
            threshold=0.5,
        )

        # With mock embeddings, exact scores are deterministic
        assert isinstance(result.has_relevant, bool)

    @pytest.mark.asyncio
    async def test_add_and_query_search(self, temp_vector_store: VectorStore):
        """Test adding to and querying search collection."""
        await temp_vector_store.add_to_search(
            ids=["test1.md#chunk_0"],
            texts=["This is chunk content for testing"],
            metadatas=[{"source_path": "test1.md", "chunk_index": 0}],
        )

        results = await temp_vector_store.query_search(
            queries=["testing content"],
            n_results=5,
        )

        assert len(results) == 1
        assert len(results[0]) == 1
        assert results[0][0].id == "test1.md#chunk_0"

    @pytest.mark.asyncio
    async def test_delete_file(self, temp_vector_store: VectorStore):
        """Test deleting a file from both collections."""
        await temp_vector_store.add_to_trigger(
            ids=["delete_me.md"],
            texts=["To be deleted"],
            metadatas=[{"source_path": "delete_me.md"}],
        )
        await temp_vector_store.add_to_search(
            ids=["delete_me.md#chunk_0", "delete_me.md#chunk_1"],
            texts=["Chunk 1", "Chunk 2"],
            metadatas=[{"source_path": "delete_me.md"}, {"source_path": "delete_me.md"}],
        )

        counts = temp_vector_store.count()
        assert counts["trigger_count"] == 1
        assert counts["search_count"] == 2

        temp_vector_store.delete("delete_me.md")

        counts = temp_vector_store.count()
        assert counts["trigger_count"] == 0
        assert counts["search_count"] == 0