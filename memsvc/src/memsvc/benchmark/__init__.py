"""Memory System RAG Recall Benchmark.

This module provides tools for benchmarking the recall performance
of the memory system's RAG retrieval capabilities.

Usage:
    # Run all benchmarks
    python -m memsvc.benchmark run --all

    # Run specific scenario
    python -m memsvc.benchmark run --scenario basic_recall

    # Compare embedding providers
    python -m memsvc.benchmark run --provider mock
"""

from memsvc.benchmark.dataset import ScenarioLoader, BenchmarkScenario
from memsvc.benchmark.metrics import MetricsCalculator, BenchmarkMetrics
from memsvc.benchmark.runner import BenchmarkRunner, BenchmarkConfig
from memsvc.benchmark.report import ReportGenerator

__all__ = [
    "ScenarioLoader",
    "BenchmarkScenario",
    "MetricsCalculator",
    "BenchmarkMetrics",
    "BenchmarkRunner",
    "BenchmarkConfig",
    "ReportGenerator",
]