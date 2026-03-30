"""Report generation for RAG benchmark.

Generates JSON and Markdown reports from benchmark results.
"""

import json
from datetime import datetime
from pathlib import Path
from typing import Any

from memsvc.benchmark.metrics import BenchmarkMetrics
from memsvc.benchmark.runner import ScenarioResult


class ReportGenerator:
    """Generate reports from benchmark results."""

    def __init__(self, output_dir: Path | None = None):
        """Initialize report generator.

        Args:
            output_dir: Directory to save reports. Defaults to current directory.
        """
        self.output_dir = output_dir or Path(".")

    def generate_json_report(
        self,
        results: list[ScenarioResult],
        output_path: Path | None = None,
    ) -> dict[str, Any]:
        """Generate JSON report.

        Args:
            results: Benchmark results.
            output_path: Path to save JSON file. If None, returns dict only.

        Returns:
            Report as dictionary.
        """
        report = {
            "generated_at": datetime.now().isoformat(),
            "total_scenarios": len(results),
            "scenarios": [],
        }

        for result in results:
            scenario_data = {
                "id": result.scenario_id,
                "name": result.scenario_name,
                "duration_ms": result.duration_ms,
                "error": result.error,
                "config": result.config,
                "metrics": self._metrics_to_dict(result.metrics),
            }
            report["scenarios"].append(scenario_data)

        if output_path:
            output_path.parent.mkdir(parents=True, exist_ok=True)
            with open(output_path, "w", encoding="utf-8") as f:
                json.dump(report, f, indent=2, ensure_ascii=False)

        return report

    def generate_markdown_report(
        self,
        results: list[ScenarioResult],
        output_path: Path | None = None,
    ) -> str:
        """Generate Markdown report.

        Args:
            results: Benchmark results.
            output_path: Path to save Markdown file. If None, returns string only.

        Returns:
            Report as Markdown string.
        """
        lines = []

        # Header
        lines.append("# Memory System RAG Benchmark Report")
        lines.append("")
        lines.append(f"**Generated**: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
        lines.append(f"**Total Scenarios**: {len(results)}")
        lines.append("")

        # Summary table
        lines.append("## Summary")
        lines.append("")
        lines.append("| Scenario | MRR | Hit Rate | Recall@5 | Precision@5 | Avg Latency |")
        lines.append("|----------|-----|----------|----------|-------------|-------------|")

        for result in results:
            if result.error:
                lines.append(f"| {result.scenario_name} | ERROR | ERROR | ERROR | ERROR | ERROR |")
            else:
                m = result.metrics
                lines.append(
                    f"| {result.scenario_name} | "
                    f"{m.mrr:.3f} | "
                    f"{m.hit_rate:.3f} | "
                    f"{m.recall_at_k.get(5, 0):.3f} | "
                    f"{m.precision_at_k.get(5, 0):.3f} | "
                    f"{m.avg_latency_ms:.1f}ms |"
                )

        lines.append("")

        # Detailed results per scenario
        lines.append("## Detailed Results")
        lines.append("")

        for result in results:
            lines.append(f"### {result.scenario_name}")
            lines.append("")

            if result.error:
                lines.append(f"**Error**: {result.error}")
                lines.append("")
                continue

            m = result.metrics

            # Core metrics
            lines.append("**Core Metrics**")
            lines.append("")
            lines.append(f"- **MRR (Mean Reciprocal Rank)**: {m.mrr:.4f}")
            lines.append(f"- **Hit Rate**: {m.hit_rate:.4f}")
            lines.append("")

            # Recall@K table
            lines.append("**Recall@K**")
            lines.append("")
            lines.append("| K | Recall | Precision |")
            lines.append("|---|--------|-----------|")
            for k in sorted(m.recall_at_k.keys()):
                lines.append(
                    f"| {k} | {m.recall_at_k[k]:.4f} | {m.precision_at_k.get(k, 0):.4f} |"
                )
            lines.append("")

            # Latency statistics
            lines.append("**Latency Statistics**")
            lines.append("")
            lines.append(f"- **Average**: {m.avg_latency_ms:.2f}ms")
            lines.append(f"- **P50**: {m.p50_latency_ms:.2f}ms")
            lines.append(f"- **P95**: {m.p95_latency_ms:.2f}ms")
            lines.append(f"- **P99**: {m.p99_latency_ms:.2f}ms")
            lines.append("")

            # Configuration
            lines.append("**Configuration**")
            lines.append("")
            for key, value in result.config.items():
                lines.append(f"- {key}: {value}")
            lines.append("")

        # Footer
        lines.append("---")
        lines.append("")
        lines.append("*Generated by OxenCode Memory System Benchmark*")
        lines.append("")

        report = "\n".join(lines)

        if output_path:
            output_path.parent.mkdir(parents=True, exist_ok=True)
            with open(output_path, "w", encoding="utf-8") as f:
                f.write(report)

        return report

    def _metrics_to_dict(self, metrics: BenchmarkMetrics) -> dict[str, Any]:
        """Convert BenchmarkMetrics to dictionary.

        Args:
            metrics: BenchmarkMetrics object.

        Returns:
            Dictionary representation.
        """
        return {
            "recall_at_k": metrics.recall_at_k,
            "precision_at_k": metrics.precision_at_k,
            "mrr": metrics.mrr,
            "hit_rate": metrics.hit_rate,
            "avg_latency_ms": metrics.avg_latency_ms,
            "p50_latency_ms": metrics.p50_latency_ms,
            "p95_latency_ms": metrics.p95_latency_ms,
            "p99_latency_ms": metrics.p99_latency_ms,
            "total_queries": metrics.total_queries,
            "total_memories": metrics.total_memories,
        }

    def save_results(
        self,
        results: list[ScenarioResult],
        output_dir: Path | None = None,
        prefix: str = "benchmark",
    ) -> tuple[Path, Path]:
        """Save both JSON and Markdown reports.

        Args:
            results: Benchmark results.
            output_dir: Output directory. Uses self.output_dir if None.
            prefix: File name prefix.

        Returns:
            Tuple of (json_path, markdown_path).
        """
        output_dir = output_dir or self.output_dir
        output_dir.mkdir(parents=True, exist_ok=True)

        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")

        json_path = output_dir / f"{prefix}_{timestamp}.json"
        md_path = output_dir / f"{prefix}_{timestamp}.md"

        self.generate_json_report(results, json_path)
        self.generate_markdown_report(results, md_path)

        # Also create a "latest" symlink/copy
        latest_json = output_dir / f"{prefix}_latest.json"
        latest_md = output_dir / f"{prefix}_latest.md"

        # Copy instead of symlink for cross-platform compatibility
        import shutil
        shutil.copy(json_path, latest_json)
        shutil.copy(md_path, latest_md)

        return json_path, md_path


def print_summary(results: list[ScenarioResult]) -> None:
    """Print a brief summary to console.

    Args:
        results: Benchmark results.
    """
    print("\n" + "=" * 60)
    print("Memory System RAG Benchmark Results")
    print("=" * 60)

    for result in results:
        print(f"\nScenario: {result.scenario_name}")

        if result.error:
            print(f"  ERROR: {result.error}")
            continue

        m = result.metrics
        print(f"  MRR: {m.mrr:.4f}")
        print(f"  Hit Rate: {m.hit_rate:.4f}")
        print(f"  Recall@5: {m.recall_at_k.get(5, 0):.4f}")
        print(f"  Precision@5: {m.precision_at_k.get(5, 0):.4f}")
        print(f"  Avg Latency: {m.avg_latency_ms:.2f}ms")
        print(f"  Total Queries: {m.total_queries}")

    print("\n" + "=" * 60)