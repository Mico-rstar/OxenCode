"""CLI entry point for memory system RAG benchmark.

Usage:
    # Run all benchmarks
    python -m memsvc.benchmark run --all

    # Run specific scenario
    python -m memsvc.benchmark run --scenario basic_recall

    # List available scenarios
    python -m memsvc.benchmark list

    # Use specific embedding provider
    python -m memsvc.benchmark run --provider mock
    python -m memsvc.benchmark run --provider qwen
"""

import argparse
import asyncio
import sys
from pathlib import Path

from memsvc.benchmark.dataset import ScenarioLoader
from memsvc.benchmark.runner import BenchmarkRunner, BenchmarkConfig, run_benchmark
from memsvc.benchmark.report import ReportGenerator, print_summary


def create_parser() -> argparse.ArgumentParser:
    """Create argument parser."""
    parser = argparse.ArgumentParser(
        prog="memsvc.benchmark",
        description="Memory System RAG Recall Benchmark",
    )

    subparsers = parser.add_subparsers(dest="command", help="Commands")

    # run command
    run_parser = subparsers.add_parser("run", help="Run benchmark")
    run_parser.add_argument(
        "--all",
        action="store_true",
        help="Run all available scenarios",
    )
    run_parser.add_argument(
        "--scenario",
        type=str,
        help="Run specific scenario by ID",
    )
    run_parser.add_argument(
        "--provider",
        type=str,
        default="mock",
        choices=["mock", "qwen"],
        help="Embedding provider to use (default: mock)",
    )
    run_parser.add_argument(
        "--output",
        "-o",
        type=Path,
        help="Output directory for reports",
    )
    run_parser.add_argument(
        "--top-k",
        type=int,
        default=5,
        help="Number of results to retrieve (default: 5)",
    )
    run_parser.add_argument(
        "--no-cleanup",
        action="store_true",
        help="Don't clean up benchmark data after running",
    )

    # list command
    list_parser = subparsers.add_parser("list", help="List available scenarios")

    return parser


def cmd_list(args: argparse.Namespace) -> int:
    """List available scenarios."""
    loader = ScenarioLoader()
    scenarios = loader.list_scenarios()

    if not scenarios:
        print("No scenarios found.")
        return 0

    print("\nAvailable Benchmark Scenarios:")
    print("-" * 60)

    for s in scenarios:
        print(f"\n  ID: {s['id']}")
        print(f"  Name: {s['name']}")
        print(f"  Description: {s['description']}")
        print(f"  Memories: {s['memory_count']}, Queries: {s['query_count']}")
        print(f"  Difficulty: {s['difficulty']}")
        if s['tags']:
            print(f"  Tags: {', '.join(s['tags'])}")

    print("-" * 60)
    print(f"\nTotal: {len(scenarios)} scenarios")

    return 0


async def cmd_run(args: argparse.Namespace) -> int:
    """Run benchmark."""
    # Determine which scenarios to run
    scenario_ids = None
    if args.scenario:
        scenario_ids = [args.scenario]
    elif not args.all:
        # Default to running all scenarios
        print("Running all scenarios (use --scenario for specific one)")
        print()

    # Create config
    config = BenchmarkConfig(
        embedding_provider=args.provider,
        top_k=args.top_k,
        output_dir=args.output,
    )

    print(f"Embedding Provider: {args.provider}")
    print(f"Top-K: {args.top_k}")
    if args.output:
        print(f"Output Directory: {args.output}")
    print()

    # Run benchmark
    results = await run_benchmark(
        scenario_ids=scenario_ids,
        embedding_provider=args.provider,
        output_dir=args.output,
        cleanup=not args.no_cleanup,
    )

    # Print summary
    print_summary(results)

    # Save reports if output directory specified
    if args.output:
        generator = ReportGenerator(args.output)
        json_path, md_path = generator.save_results(results, args.output)
        print(f"\nReports saved:")
        print(f"  JSON: {json_path}")
        print(f"  Markdown: {md_path}")

    # Return non-zero if any scenario had errors
    if any(r.error for r in results):
        return 1

    return 0


def main() -> int:
    """Main entry point."""
    parser = create_parser()
    args = parser.parse_args()

    if args.command is None:
        parser.print_help()
        return 0

    if args.command == "list":
        return cmd_list(args)

    if args.command == "run":
        return asyncio.run(cmd_run(args))

    return 0


if __name__ == "__main__":
    sys.exit(main())