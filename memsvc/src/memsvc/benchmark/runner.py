"""Benchmark runner for RAG recall evaluation.

Executes benchmark scenarios and collects metrics.
"""

import asyncio
import shutil
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from memsvc.benchmark.dataset import (
    BenchmarkScenario,
    ScenarioLoader,
    NoiseGenerator,
    MemoryEntry,
)
from memsvc.benchmark.metrics import MetricsCalculator, BenchmarkMetrics
from memsvc.core.embedding import EmbeddingProvider, get_embedding_provider
from memsvc.core.vectorstore import VectorStore


@dataclass
class BenchmarkConfig:
    """Configuration for benchmark run."""

    # Embedding provider
    embedding_provider: str = "mock"  # "qwen" or "mock"

    # Retrieval settings
    top_k: int = 5
    trigger_threshold: float = 0.7

    # Data isolation
    isolation_dir: Path | None = None  # Defaults to ~/.oxencode/benchmark_memory

    # Output
    output_dir: Path | None = None
    save_results: bool = True

    # Metrics
    k_values: list[int] | None = None  # Defaults to [1, 3, 5, 10]


@dataclass
class ScenarioResult:
    """Result of running a single scenario."""

    scenario_id: str
    scenario_name: str
    metrics: BenchmarkMetrics
    duration_ms: float
    config: dict[str, Any]
    error: str | None = None


class BenchmarkRunner:
    """Runner for RAG recall benchmarks.

    Uses isolated data directory to avoid affecting production memory.
    """

    def __init__(self, config: BenchmarkConfig | None = None):
        """Initialize benchmark runner.

        Args:
            config: Benchmark configuration. Uses defaults if not provided.
        """
        self.config = config or BenchmarkConfig()

        # Set up isolation directory
        if self.config.isolation_dir is None:
            self.config.isolation_dir = Path.home() / ".oxencode" / "benchmark_memory"

        # Initialize components
        self.scenario_loader = ScenarioLoader()
        self.metrics_calculator = MetricsCalculator(self.config.k_values)
        self.noise_generator = NoiseGenerator()

        # Vector store (initialized per scenario)
        self._vector_store: VectorStore | None = None
        self._embedding_provider: EmbeddingProvider | None = None

    def _get_isolation_dir(self) -> Path:
        """Get isolation directory, creating if needed."""
        isolation_dir = self.config.isolation_dir
        isolation_dir.mkdir(parents=True, exist_ok=True)
        return isolation_dir

    def _init_vector_store(self) -> VectorStore:
        """Initialize isolated vector store.

        Returns:
            VectorStore instance with isolated persistence.
        """
        isolation_dir = self._get_isolation_dir()
        persist_dir = isolation_dir / "chromadb"

        # Get embedding provider
        provider = get_embedding_provider(self.config.embedding_provider)

        # Create vector store with isolated persistence
        vector_store = VectorStore(
            persist_dir=persist_dir,
            embedding_provider=provider,
        )
        vector_store.initialize()

        return vector_store

    def cleanup(self) -> None:
        """Clean up benchmark data directory.

        Removes all data created during benchmark runs.
        """
        # Close vector store first to release file locks
        if self._vector_store:
            self._vector_store.close()
            self._vector_store = None

        # Force garbage collection to release any lingering references
        import gc
        gc.collect()

        if self.config.isolation_dir and self.config.isolation_dir.exists():
            # On Windows, file locks may persist briefly after close
            # Try multiple times with short delays
            import time
            for attempt in range(5):
                try:
                    shutil.rmtree(self.config.isolation_dir)
                    break
                except PermissionError:
                    if attempt < 4:
                        time.sleep(0.5)
                        gc.collect()
                    else:
                        # Final attempt - log warning and continue
                        import warnings
                        warnings.warn(f"Could not fully clean up {self.config.isolation_dir}. "
                                    f"You may need to delete it manually.")
                        # Try to delete what we can
                        try:
                            for item in self.config.isolation_dir.iterdir():
                                try:
                                    if item.is_dir():
                                        shutil.rmtree(item)
                                    else:
                                        item.unlink()
                                except Exception:
                                    pass
                        except Exception:
                            pass

    async def run_scenario(
        self,
        scenario_id: str | None = None,
        scenario: BenchmarkScenario | None = None,
    ) -> ScenarioResult:
        """Run a single benchmark scenario.

        Args:
            scenario_id: ID of scenario to load.
            scenario: Pre-loaded scenario object.

        Returns:
            ScenarioResult with metrics.
        """
        # Load scenario
        if scenario is None:
            if scenario_id is None:
                raise ValueError("Either scenario_id or scenario must be provided")
            scenario = self.scenario_loader.load_scenario(scenario_id)

        start_time = time.time()

        try:
            # Clean up any previous run data
            self.cleanup()

            # Initialize fresh vector store
            self._vector_store = self._init_vector_store()

            # Index all memories
            await self._index_memories(scenario.memories, scenario.noise_memories)

            # Run queries and collect results
            query_results = await self._run_queries(scenario.queries)

            # Calculate metrics
            metrics = self.metrics_calculator.aggregate_metrics(
                query_results,
                total_memories=len(scenario.memories) + scenario.noise_memories,
            )

            duration_ms = (time.time() - start_time) * 1000

            return ScenarioResult(
                scenario_id=scenario.id,
                scenario_name=scenario.name,
                metrics=metrics,
                duration_ms=duration_ms,
                config={
                    "embedding_provider": self.config.embedding_provider,
                    "top_k": self.config.top_k,
                    "k_values": self.config.k_values or [1, 3, 5, 10],
                },
            )

        except Exception as e:
            duration_ms = (time.time() - start_time) * 1000
            return ScenarioResult(
                scenario_id=scenario.id,
                scenario_name=scenario.name,
                metrics=BenchmarkMetrics(),
                duration_ms=duration_ms,
                config={},
                error=str(e),
            )

    async def run_all_scenarios(self) -> list[ScenarioResult]:
        """Run all available scenarios.

        Returns:
            List of ScenarioResult objects.
        """
        scenarios = self.scenario_loader.load_all_scenarios()
        results = []

        for scenario in scenarios:
            result = await self.run_scenario(scenario=scenario)
            results.append(result)

        return results

    async def _index_memories(
        self,
        memories: list[MemoryEntry],
        noise_count: int = 0,
    ) -> None:
        """Index memories into vector store.

        Args:
            memories: Memory entries to index.
            noise_count: Number of noise memories to add.
        """
        if not self._vector_store:
            raise RuntimeError("Vector store not initialized")

        # Combine scenario memories with noise
        all_memories = list(memories)

        if noise_count > 0:
            noise_memories = self.noise_generator.generate_noise_memories(noise_count)
            all_memories.extend(noise_memories)

        # Batch size for embedding API (DashScope limit is 10)
        BATCH_SIZE = 10

        # Add to trigger collection in batches
        for i in range(0, len(all_memories), BATCH_SIZE):
            batch = all_memories[i:i + BATCH_SIZE]
            trigger_ids = [m.id for m in batch]
            trigger_texts = [m.description for m in batch]
            trigger_metadatas = [
                {
                    "category": m.category,
                    "content_preview": m.content[:200] if len(m.content) > 200 else m.content,
                }
                for m in batch
            ]
            await self._vector_store.add_to_trigger(
                ids=trigger_ids,
                texts=trigger_texts,
                metadatas=trigger_metadatas,
            )

        # Prepare data for search collection (content chunks)
        # For simplicity, we use the full content as one chunk
        # In production, this would use the chunker
        search_ids = []
        search_texts = []
        search_metadatas = []

        for m in all_memories:
            # Create a single chunk per memory
            search_ids.append(f"{m.id}#chunk_0")
            search_texts.append(m.content)
            search_metadatas.append({
                "source_path": m.id,
                "description": m.description,
                "category": m.category,
            })

        # Add to search collection in batches
        for i in range(0, len(search_ids), BATCH_SIZE):
            batch_ids = search_ids[i:i + BATCH_SIZE]
            batch_texts = search_texts[i:i + BATCH_SIZE]
            batch_metadatas = search_metadatas[i:i + BATCH_SIZE]

            await self._vector_store.add_to_search(
                ids=batch_ids,
                texts=batch_texts,
                metadatas=batch_metadatas,
            )

    async def _run_queries(self, queries: list) -> list:
        """Run queries and measure retrieval performance.

        Args:
            queries: List of QueryEntry objects.

        Returns:
            List of QueryResult objects.
        """
        from memsvc.benchmark.metrics import QueryResult

        if not self._vector_store:
            raise RuntimeError("Vector store not initialized")

        results = []

        for query in queries:
            start_time = time.time()

            # Execute search
            search_results = await self._vector_store.query_search(
                queries=[query.text],
                n_results=self.config.top_k,
            )

            latency_ms = (time.time() - start_time) * 1000

            # Extract retrieved IDs
            retrieved_ids = []
            scores = []
            if search_results and search_results[0]:
                for result in search_results[0]:
                    # Extract source_path from metadata
                    retrieved_ids.append(result.metadata.get("source_path", result.id))
                    scores.append(result.score)

            # Calculate query metrics
            query_result = self.metrics_calculator.calculate_query_metrics(
                query_id=query.id,
                query_text=query.text,
                expected_ids=query.expected_ids,
                retrieved_ids=retrieved_ids,
                scores=scores,
                latency_ms=latency_ms,
            )

            results.append(query_result)

        return results


async def run_benchmark(
    scenario_ids: list[str] | None = None,
    embedding_provider: str = "mock",
    output_dir: Path | None = None,
    cleanup: bool = True,
) -> list[ScenarioResult]:
    """Convenience function to run benchmark.

    Args:
        scenario_ids: Specific scenarios to run. Runs all if None.
        embedding_provider: Embedding provider to use.
        output_dir: Directory for output reports.
        cleanup: Whether to clean up benchmark data after running.

    Returns:
        List of ScenarioResult objects.
    """
    config = BenchmarkConfig(
        embedding_provider=embedding_provider,
        output_dir=output_dir,
    )

    runner = BenchmarkRunner(config)

    try:
        if scenario_ids:
            results = []
            for scenario_id in scenario_ids:
                result = await runner.run_scenario(scenario_id)
                results.append(result)
        else:
            results = await runner.run_all_scenarios()

        return results

    finally:
        if cleanup:
            runner.cleanup()