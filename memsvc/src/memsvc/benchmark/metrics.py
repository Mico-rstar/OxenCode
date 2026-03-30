"""Metrics calculation for RAG benchmark.

Provides standard retrieval metrics: Recall@K, Precision@K, MRR, Hit Rate.
"""

from dataclasses import dataclass, field
from typing import Any
import time


@dataclass
class QueryResult:
    """Result of a single query evaluation."""

    query_id: str
    query_text: str
    expected_ids: list[str]
    retrieved_ids: list[str]
    scores: list[float]  # Similarity scores for retrieved items
    latency_ms: float

    # Computed metrics for this query
    recall_at_k: dict[int, float] = field(default_factory=dict)
    precision_at_k: dict[int, float] = field(default_factory=dict)
    reciprocal_rank: float = 0.0
    hit: bool = False


@dataclass
class BenchmarkMetrics:
    """Aggregated metrics across all queries."""

    # Core metrics
    recall_at_k: dict[int, float] = field(default_factory=dict)
    precision_at_k: dict[int, float] = field(default_factory=dict)
    mrr: float = 0.0  # Mean Reciprocal Rank
    hit_rate: float = 0.0

    # Latency statistics
    avg_latency_ms: float = 0.0
    p50_latency_ms: float = 0.0
    p95_latency_ms: float = 0.0
    p99_latency_ms: float = 0.0

    # Details
    total_queries: int = 0
    total_memories: int = 0
    query_results: list[QueryResult] = field(default_factory=list)

    # Per-category metrics (if categories are used)
    category_metrics: dict[str, "BenchmarkMetrics"] = field(default_factory=dict)


class MetricsCalculator:
    """Calculate retrieval metrics for benchmark results."""

    def __init__(self, k_values: list[int] | None = None):
        """Initialize calculator.

        Args:
            k_values: List of K values for Recall@K and Precision@K.
                     Defaults to [1, 3, 5, 10].
        """
        self.k_values = k_values or [1, 3, 5, 10]

    def calculate_query_metrics(
        self,
        query_id: str,
        query_text: str,
        expected_ids: list[str],
        retrieved_ids: list[str],
        scores: list[float],
        latency_ms: float,
    ) -> QueryResult:
        """Calculate metrics for a single query.

        Args:
            query_id: Query identifier.
            query_text: The query text.
            expected_ids: Ground truth relevant IDs.
            retrieved_ids: IDs returned by the retrieval system.
            scores: Similarity scores for retrieved items.
            latency_ms: Query latency in milliseconds.

        Returns:
            QueryResult with computed metrics.
        """
        result = QueryResult(
            query_id=query_id,
            query_text=query_text,
            expected_ids=expected_ids,
            retrieved_ids=retrieved_ids,
            scores=scores,
            latency_ms=latency_ms,
        )

        # Calculate Recall@K and Precision@K for each K value
        for k in self.k_values:
            result.recall_at_k[k] = self._recall_at_k(
                expected_ids, retrieved_ids, k
            )
            result.precision_at_k[k] = self._precision_at_k(
                expected_ids, retrieved_ids, k
            )

        # Calculate reciprocal rank
        result.reciprocal_rank = self._reciprocal_rank(expected_ids, retrieved_ids)

        # Check for hit (at least one relevant item retrieved)
        result.hit = any(rid in expected_ids for rid in retrieved_ids)

        return result

    def aggregate_metrics(
        self,
        query_results: list[QueryResult],
        total_memories: int = 0,
    ) -> BenchmarkMetrics:
        """Aggregate metrics across all queries.

        Args:
            query_results: List of query results.
            total_memories: Total number of memories in the index.

        Returns:
            Aggregated BenchmarkMetrics.
        """
        if not query_results:
            return BenchmarkMetrics(total_memories=total_memories)

        metrics = BenchmarkMetrics(
            total_queries=len(query_results),
            total_memories=total_memories,
            query_results=query_results,
        )

        # Aggregate Recall@K and Precision@K
        for k in self.k_values:
            recalls = [r.recall_at_k.get(k, 0.0) for r in query_results]
            precisions = [r.precision_at_k.get(k, 0.0) for r in query_results]

            metrics.recall_at_k[k] = sum(recalls) / len(recalls)
            metrics.precision_at_k[k] = sum(precisions) / len(precisions)

        # Calculate MRR
        reciprocal_ranks = [r.reciprocal_rank for r in query_results]
        metrics.mrr = sum(reciprocal_ranks) / len(reciprocal_ranks)

        # Calculate Hit Rate
        hits = sum(1 for r in query_results if r.hit)
        metrics.hit_rate = hits / len(query_results)

        # Calculate latency statistics
        latencies = sorted([r.latency_ms for r in query_results])
        metrics.avg_latency_ms = sum(latencies) / len(latencies)
        metrics.p50_latency_ms = self._percentile(latencies, 50)
        metrics.p95_latency_ms = self._percentile(latencies, 95)
        metrics.p99_latency_ms = self._percentile(latencies, 99)

        return metrics

    def _recall_at_k(
        self,
        expected_ids: list[str],
        retrieved_ids: list[str],
        k: int,
    ) -> float:
        """Calculate Recall@K.

        Recall@K = |relevant ∩ retrieved_k| / |relevant|

        Args:
            expected_ids: Ground truth relevant IDs.
            retrieved_ids: IDs returned by the retrieval system.
            k: Number of top results to consider.

        Returns:
            Recall@K value.
        """
        if not expected_ids:
            return 0.0

        top_k = retrieved_ids[:k]
        relevant_in_top_k = sum(1 for rid in top_k if rid in expected_ids)

        return relevant_in_top_k / len(expected_ids)

    def _precision_at_k(
        self,
        expected_ids: list[str],
        retrieved_ids: list[str],
        k: int,
    ) -> float:
        """Calculate Precision@K.

        Precision@K = |relevant ∩ retrieved_k| / K

        Args:
            expected_ids: Ground truth relevant IDs.
            retrieved_ids: IDs returned by the retrieval system.
            k: Number of top results to consider.

        Returns:
            Precision@K value.
        """
        if k == 0:
            return 0.0

        top_k = retrieved_ids[:k]
        relevant_in_top_k = sum(1 for rid in top_k if rid in expected_ids)

        return relevant_in_top_k / k

    def _reciprocal_rank(
        self,
        expected_ids: list[str],
        retrieved_ids: list[str],
    ) -> float:
        """Calculate Reciprocal Rank.

        RR = 1 / rank of first relevant item

        Args:
            expected_ids: Ground truth relevant IDs.
            retrieved_ids: IDs returned by the retrieval system.

        Returns:
            Reciprocal rank value (0 if no relevant item found).
        """
        for i, rid in enumerate(retrieved_ids):
            if rid in expected_ids:
                return 1.0 / (i + 1)

        return 0.0

    def _percentile(self, sorted_values: list[float], p: float) -> float:
        """Calculate percentile of sorted values.

        Args:
            sorted_values: Sorted list of values.
            p: Percentile (0-100).

        Returns:
            Percentile value.
        """
        if not sorted_values:
            return 0.0

        index = (len(sorted_values) - 1) * p / 100.0
        lower = int(index)
        upper = lower + 1

        if upper >= len(sorted_values):
            return sorted_values[-1]

        weight = index - lower
        return sorted_values[lower] * (1 - weight) + sorted_values[upper] * weight